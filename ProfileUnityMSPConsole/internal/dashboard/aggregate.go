package dashboard

import (
	"context"
	"fmt"
	"time"

	"profileunity-msp-console/internal/snapshot"
	"profileunity-msp-console/internal/tenant"
)

// Repos bundles what BuildAll needs. Kept minimal and interface-free
// since both concrete repos already live in this module and nothing else
// implements them — no reason to abstract further yet.
type Repos struct {
	Tenants   *tenant.Repo
	Snapshots *snapshot.Repo
}

// BuildAll computes TenantStatus for every registered tenant, in two
// queries total (not one-per-tenant) regardless of tenant count.
func BuildAll(ctx context.Context, repos Repos, now time.Time, loc *time.Location) ([]TenantStatus, error) {
	tenants, err := repos.Tenants.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard: list tenants: %w", err)
	}

	latest, err := repos.Snapshots.LatestForAllTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard: latest snapshots: %w", err)
	}
	latestSuccess, err := repos.Snapshots.LatestSuccessForAllTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard: latest successful snapshots: %w", err)
	}

	result := make([]TenantStatus, 0, len(tenants))
	for _, t := range tenants {
		var l, ls *snapshot.Snapshot
		if v, ok := latest[t.ID]; ok {
			l = &v
		}
		if v, ok := latestSuccess[t.ID]; ok {
			ls = &v
		}
		result = append(result, Compute(t, l, ls, now, loc))
	}
	return result, nil
}
