import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import {
  LicenseServerConnection,
  LicenseServerWriteRequest,
  DecodedLicense,
  LicensePushResult,
  LicensePushRecord,
} from './models/license';

@Injectable({ providedIn: 'root' })
export class LicensesService {
  private readonly http = inject(HttpClient);

  getConnection(tenantId: string): Promise<LicenseServerConnection> {
    return firstValueFrom(this.http.get<LicenseServerConnection>(`/api/tenants/${tenantId}/license-server`));
  }

  saveConnection(tenantId: string, req: LicenseServerWriteRequest): Promise<LicenseServerConnection> {
    return firstValueFrom(this.http.put<LicenseServerConnection>(`/api/tenants/${tenantId}/license-server`, req));
  }

  /** Tests reachability using whatever hostname/port is passed in --
   * not the tenant's saved connection -- since the server's own
   * /api/checkup is unauthenticated and needs no saved credential.
   * Mirrors the Tenants screen's Test Connection, which likewise tests
   * live form values rather than requiring a save first. */
  checkup(tenantId: string, req: { hostname: string; port: number; tlsSkipVerify: boolean }): Promise<{ ok: boolean; message: string }> {
    return firstValueFrom(this.http.post<{ ok: boolean; message: string }>(`/api/tenants/${tenantId}/license-server/checkup`, req));
  }

  /** Decodes a license code locally on the server (no network call to
   * the tenant's License Server) so it can be reviewed before pushing. */
  preview(tenantId: string, licenseBase64: string): Promise<DecodedLicense> {
    return firstValueFrom(this.http.post<DecodedLicense>(`/api/tenants/${tenantId}/license/preview`, { licenseBase64 }));
  }

  /** Installs a license on the tenant's License Server -- a destructive
   * replace. confirm must be true or the server rejects the request. */
  push(tenantId: string, licenseBase64: string, confirm: boolean): Promise<LicensePushResult> {
    return firstValueFrom(this.http.post<LicensePushResult>(`/api/tenants/${tenantId}/license/push`, { licenseBase64, confirm }));
  }

  history(tenantId: string): Promise<LicensePushRecord[]> {
    return firstValueFrom(this.http.get<LicensePushRecord[]>(`/api/tenants/${tenantId}/license/history`));
  }
}
