import { Tenant } from './tenant';

export type UsageStatus = 'good' | 'fair' | 'poor' | 'unknown';
export type ExpiryStatus = 'ok' | 'expiring_soon' | 'expired' | 'unknown';
export type DataStatus = 'ok' | 'stale' | 'failing' | 'never_collected';

export interface TenantStatus {
  tenant: Tenant;
  dataStatus: DataStatus;
  usageStatus: UsageStatus;
  expiryStatus: ExpiryStatus;
  utilizationPercent: number | null;
  expiryRunwayDays: number | null;
  licenseMode?: string;
  licenseProduct?: string;
  consoleVersion?: string;
  totalLicenses?: number;
  usedLicenses?: number;
  lastSuccessAtUtc?: string;
  lastAttemptAtUtc?: string;
  lastError?: string;
}
