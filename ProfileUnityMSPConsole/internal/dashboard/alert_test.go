package dashboard

import "testing"

func TestDetectAlerts_HealthyTenantIsOmitted(t *testing.T) {
	all := []TenantStatus{{Usage: UsageGood, Expiry: ExpiryOK, Data: DataOK}}
	if got := DetectAlerts(all); len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

func TestDetectAlerts_UsagePoor(t *testing.T) {
	all := []TenantStatus{{Usage: UsagePoor, Expiry: ExpiryOK, Data: DataOK}}
	got := DetectAlerts(all)
	if len(got) != 1 || len(got[0].Reasons) != 1 || got[0].Reasons[0] != AlertUsagePoor {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectAlerts_UsageFairIsNotAlertable(t *testing.T) {
	all := []TenantStatus{{Usage: UsageFair, Expiry: ExpiryOK, Data: DataOK}}
	if got := DetectAlerts(all); len(got) != 0 {
		t.Errorf("got %+v, want none (fair/near-limit is not itself alertable)", got)
	}
}

func TestDetectAlerts_Expired(t *testing.T) {
	all := []TenantStatus{{Usage: UsageGood, Expiry: ExpiryExpired, Data: DataOK}}
	got := DetectAlerts(all)
	if len(got) != 1 || got[0].Reasons[0] != AlertExpired {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectAlerts_ExpiringSoon(t *testing.T) {
	all := []TenantStatus{{Usage: UsageGood, Expiry: ExpiryExpiringSoon, Data: DataOK}}
	got := DetectAlerts(all)
	if len(got) != 1 || got[0].Reasons[0] != AlertExpiringSoon {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectAlerts_DataNotOK(t *testing.T) {
	for _, d := range []DataStatus{DataStale, DataFailing, DataNeverCollected} {
		all := []TenantStatus{{Usage: UsageGood, Expiry: ExpiryOK, Data: d}}
		got := DetectAlerts(all)
		if len(got) != 1 || got[0].Reasons[0] != AlertDataNotOK {
			t.Fatalf("data=%v: got %+v", d, got)
		}
	}
}

func TestDetectAlerts_MultipleReasonsOnOneTenant(t *testing.T) {
	all := []TenantStatus{{Usage: UsagePoor, Expiry: ExpiryExpired, Data: DataFailing}}
	got := DetectAlerts(all)
	if len(got) != 1 {
		t.Fatalf("got %d alerts, want 1", len(got))
	}
	if len(got[0].Reasons) != 3 {
		t.Fatalf("got %d reasons, want 3: %+v", len(got[0].Reasons), got[0].Reasons)
	}
}

func TestDetectAlerts_MixedTenantsOnlyAlertableReturned(t *testing.T) {
	all := []TenantStatus{
		{Usage: UsageGood, Expiry: ExpiryOK, Data: DataOK},
		{Usage: UsagePoor, Expiry: ExpiryOK, Data: DataOK},
		{Usage: UsageGood, Expiry: ExpiryOK, Data: DataOK},
	}
	got := DetectAlerts(all)
	if len(got) != 1 {
		t.Fatalf("got %d alerts, want 1", len(got))
	}
}
