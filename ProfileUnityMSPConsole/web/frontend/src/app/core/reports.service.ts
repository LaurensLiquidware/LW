import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { PortfolioMonthlyReport, TenantMonthlyReport } from './models/report';

@Injectable({ providedIn: 'root' })
export class ReportsService {
  private readonly http = inject(HttpClient);

  tenantMonthlyReport(tenantId: string, year: number, month: number): Promise<TenantMonthlyReport> {
    return firstValueFrom(
      this.http.get<TenantMonthlyReport>(`/api/tenants/${tenantId}/reports/monthly`, { params: new HttpParams({ fromObject: { year, month } }) }),
    );
  }

  portfolioMonthlyReport(year: number, month: number): Promise<PortfolioMonthlyReport> {
    return firstValueFrom(
      this.http.get<PortfolioMonthlyReport>('/api/reports/portfolio/monthly', { params: new HttpParams({ fromObject: { year, month } }) }),
    );
  }

  // Plain URLs, not blob-fetched: the PDF endpoints need only the session
  // cookie (no CSRF, since they're GETs), so a same-origin <a href> in the
  // template downloads them directly -- the same pattern the About screen
  // already uses for Spark_License.pdf/bom.cdx.json.
  tenantMonthlyReportPdfUrl(tenantId: string, year: number, month: number): string {
    return `/api/tenants/${tenantId}/reports/monthly.pdf?year=${year}&month=${month}`;
  }

  portfolioMonthlyReportPdfUrl(year: number, month: number): string {
    return `/api/reports/portfolio/monthly.pdf?year=${year}&month=${month}`;
  }
}
