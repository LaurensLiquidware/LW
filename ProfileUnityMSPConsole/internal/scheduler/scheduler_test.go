package scheduler

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"profileunity-msp-console/internal/db"
	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

const licenseInfoSuccessJSON = `{ "WebMessageType": 2, "Type": "success", "Message": "", "MessageKey": null, "Tag": [ {
	"RegisteredTo": "Test Tenant", "LicenseMode": "NamedUser", "LicenseProduct": "ProU+FlexApp",
	"SupportEnds": "12/31/2026", "TotalLicenses": "5", "UsedLicenses": "1", "Evaluation": "Yes",
	"ConsoleVersion": "6.9.5.9678 3038806 2026-07-01", "IsTrialExpired": "false", "IsTrial": "false",
	"IsProUOnly": "false", "IsFlexOnly": "false"
} ] }`

func newTestRepos(t *testing.T) (*tenant.Repo, *snapshot.Repo) {
	t.Helper()
	sqlDB, err := db.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return tenant.NewRepo(sqlDB, nil), snapshot.NewRepo(sqlDB)
}

func hostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host, portStr, err := net.SplitHostPort(u)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func TestScheduler_CollectNow_MixedOutcomes(t *testing.T) {
	tenantRepo, snapshotRepo := newTestRepos(t)
	ctx := context.Background()

	healthy := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer healthy.Close()
	healthyHost, healthyPort := hostPort(t, healthy.URL)

	deadListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := deadListener.Addr().(*net.TCPAddr)
	deadListener.Close()

	healthyTenant, err := tenantRepo.Create(ctx, tenant.CreateInput{
		DisplayName: "Healthy", Hostname: healthyHost, Port: healthyPort, TLSSkipVerify: true, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadTenant, err := tenantRepo.Create(ctx, tenant.CreateInput{
		DisplayName: "Dead", Hostname: "127.0.0.1", Port: deadAddr.Port, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tenantRepo.Create(ctx, tenant.CreateInput{
		DisplayName: "Disabled", Hostname: healthyHost, Port: healthyPort, TLSSkipVerify: true, Enabled: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	sched := New(tenantRepo, snapshotRepo, time.Hour, time.UTC, 5, 5*time.Second)
	summary, err := sched.CollectNow(ctx)
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}

	if summary.TenantCount != 2 {
		t.Errorf("TenantCount = %d, want 2 (disabled tenant must be skipped)", summary.TenantCount)
	}
	if summary.Counts[snapshot.StatusSuccess] != 1 {
		t.Errorf("success count = %d, want 1", summary.Counts[snapshot.StatusSuccess])
	}
	if summary.Counts[snapshot.StatusUnreachable] != 1 {
		t.Errorf("unreachable count = %d, want 1", summary.Counts[snapshot.StatusUnreachable])
	}

	status := sched.Status()
	if status.Running {
		t.Error("Running should be false after CollectNow returns")
	}
	if status.LastRunOutcome != "partial" {
		t.Errorf("LastRunOutcome = %q, want partial", status.LastRunOutcome)
	}

	healthySnap, err := snapshotRepo.GetByTenantAndDate(ctx, healthyTenant.ID, snapshot.CollectionDateFor(summary.StartedAt, time.UTC))
	if err != nil {
		t.Fatalf("GetByTenantAndDate healthy: %v", err)
	}
	if healthySnap.Status != snapshot.StatusSuccess || healthySnap.UsedLicenses == nil {
		t.Errorf("got %+v", healthySnap)
	}

	deadSnap, err := snapshotRepo.GetByTenantAndDate(ctx, deadTenant.ID, snapshot.CollectionDateFor(summary.StartedAt, time.UTC))
	if err != nil {
		t.Fatalf("GetByTenantAndDate dead: %v", err)
	}
	if deadSnap.Status != snapshot.StatusUnreachable || deadSnap.UsedLicenses != nil {
		t.Errorf("dead tenant snapshot should be unreachable with no license figures, got %+v", deadSnap)
	}
}

func TestScheduler_CollectNow_IsIdempotentAcrossRuns(t *testing.T) {
	tenantRepo, snapshotRepo := newTestRepos(t)
	ctx := context.Background()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer srv.Close()
	host, port := hostPort(t, srv.URL)

	tn, err := tenantRepo.Create(ctx, tenant.CreateInput{DisplayName: "x", Hostname: host, Port: port, TLSSkipVerify: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	sched := New(tenantRepo, snapshotRepo, time.Hour, time.UTC, 5, 5*time.Second)
	if _, err := sched.CollectNow(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := sched.CollectNow(ctx); err != nil {
		t.Fatal(err)
	}

	rows, err := snapshotRepo.ListByTenant(ctx, tn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("got %d snapshot rows after two runs on the same day, want 1", len(rows))
	}
}

func TestScheduler_CollectNow_RespectsConcurrencyCap(t *testing.T) {
	tenantRepo, snapshotRepo := newTestRepos(t)
	ctx := context.Background()

	var current, maxSeen int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&current, 1)
		for {
			m := atomic.LoadInt32(&maxSeen)
			if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		w.Write([]byte(licenseInfoSuccessJSON))
	}))
	defer srv.Close()
	host, port := hostPort(t, srv.URL)

	const concurrencyCap = 2
	for i := 0; i < 6; i++ {
		_, err := tenantRepo.Create(ctx, tenant.CreateInput{
			DisplayName: "tenant", Hostname: host, Port: port, TLSSkipVerify: true, Enabled: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	sched := New(tenantRepo, snapshotRepo, time.Hour, time.UTC, concurrencyCap, 5*time.Second)
	if _, err := sched.CollectNow(ctx); err != nil {
		t.Fatal(err)
	}

	if got := atomic.LoadInt32(&maxSeen); got > int32(concurrencyCap) {
		t.Errorf("max concurrent requests = %d, want <= %d", got, concurrencyCap)
	}
}
