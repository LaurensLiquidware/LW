import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { Tenant, TenantWriteRequest, TestConnectionRequest, TestConnectionResponse } from './models/tenant';
import { TenantStatus } from './models/dashboard';

@Injectable({ providedIn: 'root' })
export class TenantsService {
  private readonly http = inject(HttpClient);

  list(): Promise<Tenant[]> {
    return firstValueFrom(this.http.get<Tenant[]>('/api/tenants'));
  }

  get(id: string): Promise<Tenant> {
    return firstValueFrom(this.http.get<Tenant>(`/api/tenants/${id}`));
  }

  create(req: TenantWriteRequest): Promise<Tenant> {
    return firstValueFrom(this.http.post<Tenant>('/api/tenants', req));
  }

  update(id: string, req: TenantWriteRequest): Promise<Tenant> {
    return firstValueFrom(this.http.put<Tenant>(`/api/tenants/${id}`, req));
  }

  delete(id: string): Promise<void> {
    return firstValueFrom(this.http.delete<void>(`/api/tenants/${id}`));
  }

  testConnection(req: TestConnectionRequest): Promise<TestConnectionResponse> {
    return firstValueFrom(this.http.post<TestConnectionResponse>('/api/tenants/test', req));
  }

  dashboard(): Promise<TenantStatus[]> {
    return firstValueFrom(this.http.get<TenantStatus[]>('/api/dashboard'));
  }
}
