import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { PortfolioPoint, TenantHistory } from './models/history';

@Injectable({ providedIn: 'root' })
export class HistoryService {
  private readonly http = inject(HttpClient);

  tenantHistory(id: string): Promise<TenantHistory> {
    return firstValueFrom(this.http.get<TenantHistory>(`/api/tenants/${id}/history`));
  }

  portfolioHistory(): Promise<PortfolioPoint[]> {
    return firstValueFrom(this.http.get<PortfolioPoint[]>('/api/history/portfolio'));
  }
}
