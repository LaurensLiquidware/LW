export type AlertReason = 'usage_poor' | 'expiry_expired' | 'expiry_expiring_soon' | 'data_not_ok';

export interface TenantStatusSummary {
  tenant: { id: string; displayName: string };
  dataStatus: string;
  usageStatus: string;
  expiryStatus: string;
}

export interface Alert {
  tenant: TenantStatusSummary;
  reasons: AlertReason[];
}
