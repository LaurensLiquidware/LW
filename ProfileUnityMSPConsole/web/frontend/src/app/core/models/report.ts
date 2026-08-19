import { Tenant } from './tenant';
import { EntitlementChange } from './history';

export type Coverage = 'complete' | 'partial' | 'none';

export interface TenantMonthlyReport {
  tenant: Tenant;
  year: number;
  month: number;
  daysInMonth: number;
  daysCollected: number;
  daysFailed: number;
  daysNeverAttempted: number;
  coverage: Coverage;
  peakUsed: number | null;
  peakUsedDate?: string;
  averageUsed: number | null;
  entitledAtMonthEnd: number | null;
  maximumUsersAtMonthEnd: number | null;
  maximumUsersUnlimited: boolean;
  licenseProductAtMonthEnd?: string;
  entitlementChanges: EntitlementChange[];
}

export interface PortfolioMonthlyReport {
  year: number;
  month: number;
  tenantsRegistered: number;
  peakTotalUsed: number | null;
  peakTotalUsedDate?: string;
  averageTotalUsed: number | null;
  totalEntitledAtMonthEnd: number | null;
  totalMaximumUsersAtMonthEnd: number | null;
  tenantsUnlimitedAtMonthEnd: number;
  tenantReports: TenantMonthlyReport[];
}
