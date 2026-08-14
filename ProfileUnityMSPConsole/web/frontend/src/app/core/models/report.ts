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
  tenantReports: TenantMonthlyReport[];
}
