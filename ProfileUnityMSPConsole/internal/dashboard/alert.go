package dashboard

// AlertReason is why a tenant is alertable. A tenant can have more than
// one reason at once (e.g. both over its license limit and its data is
// stale) — Alert carries all of them rather than picking just one, so the
// operator sees the full picture in one place.
type AlertReason string

const (
	AlertUsagePoor    AlertReason = "usage_poor"
	AlertExpired      AlertReason = "expiry_expired"
	AlertExpiringSoon AlertReason = "expiry_expiring_soon"
	AlertDataNotOK    AlertReason = "data_not_ok"
)

// Alert is one tenant's alertable state.
type Alert struct {
	Tenant  TenantStatus
	Reasons []AlertReason
}

// DetectAlerts scans every tenant's computed status and returns the
// subset that needs operator attention: usage at/over the license limit,
// support entitlement expired or expiring soon, or data that can't
// currently be trusted (failing, stale, or never collected) — the last
// of these is arguably the most urgent, since it means the other two
// statuses for that tenant might already be wrong and nobody would know.
// A tenant with no alertable condition is simply omitted, not included
// with an empty Reasons list.
func DetectAlerts(all []TenantStatus) []Alert {
	var alerts []Alert
	for _, ts := range all {
		var reasons []AlertReason
		if ts.Usage == UsagePoor {
			reasons = append(reasons, AlertUsagePoor)
		}
		switch ts.Expiry {
		case ExpiryExpired:
			reasons = append(reasons, AlertExpired)
		case ExpiryExpiringSoon:
			reasons = append(reasons, AlertExpiringSoon)
		}
		if ts.Data != DataOK {
			reasons = append(reasons, AlertDataNotOK)
		}
		if len(reasons) > 0 {
			alerts = append(alerts, Alert{Tenant: ts, Reasons: reasons})
		}
	}
	return alerts
}
