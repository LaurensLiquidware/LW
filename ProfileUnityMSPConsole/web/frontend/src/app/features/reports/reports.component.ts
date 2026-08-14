import { Component, OnInit, inject, signal } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { SelectModule } from 'primeng/select';
import { SelectButtonModule } from 'primeng/selectbutton';
import { InputNumberModule } from 'primeng/inputnumber';
import { PrimeTemplate } from 'primeng/api';

import { Tenant } from '../../core/models/tenant';
import { TenantsService } from '../../core/tenants.service';
import { ReportsService } from '../../core/reports.service';
import { PortfolioMonthlyReport, TenantMonthlyReport } from '../../core/models/report';
import { StatusBadgeComponent } from '../../shared/status-badge.component';

type Mode = 'tenant' | 'portfolio';

/**
 * Monthly reports (project brief §7.5). Every figure comes from the same
 * pure functions the History screen's data flows through on the backend
 * (BuildTenantMonthlyReport/BuildPortfolioMonthlyReport), so a report
 * never disagrees with what the History chart already showed. Coverage
 * is reported explicitly and rendered with the neutral (non-GFP)
 * StatusBadgeComponent styling, same reasoning as the dashboard's "data"
 * status: a partially-collected month must never look merely "poor".
 */
@Component({
  selector: 'app-reports',
  standalone: true,
  imports: [FormsModule, TranslocoModule, SelectModule, SelectButtonModule, InputNumberModule, PrimeTemplate, StatusBadgeComponent, DecimalPipe],
  templateUrl: './reports.component.html',
})
export class ReportsComponent implements OnInit {
  private readonly tenantsService = inject(TenantsService);
  private readonly reportsService = inject(ReportsService);
  private readonly transloco = inject(TranslocoService);

  // Static, referentially-stable arrays: PrimeNG's SelectButton/Select
  // components re-run their own change detection whenever [options] gets
  // a *new array reference*, which -- as the History screen's mode
  // toggle found the hard way -- turns into an infinite change-detection
  // loop if that reference is rebuilt every CD cycle (e.g. from a getter
  // or a translate() call in a template expression). Labels are looked
  // up per-item via monthLabel()/modeLabel(), not baked into these
  // arrays, so re-rendering on a language switch never touches the
  // [options] identity itself.
  readonly modes: { labelKey: string; value: Mode }[] = [
    { labelKey: 'reports.perTenant', value: 'tenant' },
    { labelKey: 'reports.portfolio', value: 'portfolio' },
  ];
  readonly mode = signal<Mode>('tenant');

  readonly months: number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];

  monthLabel(month: number): string {
    const date = new Date(2000, month - 1, 1);
    return new Intl.DateTimeFormat(this.transloco.getActiveLang(), { month: 'long' }).format(date);
  }

  readonly tenants = signal<Tenant[]>([]);
  readonly selectedTenant = signal<Tenant | null>(null);
  readonly year = signal(new Date().getFullYear());
  readonly month = signal(new Date().getMonth() + 1);

  readonly loading = signal(false);
  readonly tenantReport = signal<TenantMonthlyReport | null>(null);
  readonly portfolioReport = signal<PortfolioMonthlyReport | null>(null);

  async ngOnInit(): Promise<void> {
    this.tenants.set(await this.tenantsService.list());
    if (this.tenants().length > 0) {
      this.selectedTenant.set(this.tenants()[0]);
    }
    await this.load();
  }

  async setMode(mode: Mode): Promise<void> {
    this.mode.set(mode);
    await this.load();
  }

  async onTenantChange(): Promise<void> {
    await this.load();
  }

  async onMonthChange(): Promise<void> {
    await this.load();
  }

  async load(): Promise<void> {
    this.loading.set(true);
    this.tenantReport.set(null);
    this.portfolioReport.set(null);
    try {
      if (this.mode() === 'tenant') {
        const tenant = this.selectedTenant();
        if (!tenant) {
          return;
        }
        this.tenantReport.set(await this.reportsService.tenantMonthlyReport(tenant.id, this.year(), this.month()));
      } else {
        this.portfolioReport.set(await this.reportsService.portfolioMonthlyReport(this.year(), this.month()));
      }
    } finally {
      this.loading.set(false);
    }
  }

  get tenantPdfUrl(): string {
    const tenant = this.selectedTenant();
    return tenant ? this.reportsService.tenantMonthlyReportPdfUrl(tenant.id, this.year(), this.month()) : '';
  }

  get portfolioPdfUrl(): string {
    return this.reportsService.portfolioMonthlyReportPdfUrl(this.year(), this.month());
  }
}
