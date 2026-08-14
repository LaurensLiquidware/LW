import { Tenant } from './tenant';

export interface HistoryPoint {
  date: string;
  status: string;
  usedLicenses: number | null;
  totalLicenses: number | null;
  errorMessage?: string;
}

export interface EntitlementChange {
  date: string;
  fromTotal: number;
  toTotal: number;
}

export interface TenantHistory {
  tenant: Tenant;
  points: HistoryPoint[];
  entitlementChanges: EntitlementChange[];
}

export interface PortfolioPoint {
  date: string;
  totalUsed: number;
  totalEntitled: number;
  tenantsReporting: number;
  tenantsRegistered: number;
}
