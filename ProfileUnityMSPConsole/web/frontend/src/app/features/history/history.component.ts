import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { ChartModule } from 'primeng/chart';
import { SelectModule } from 'primeng/select';
import { SelectButtonModule } from 'primeng/selectbutton';
import { PrimeTemplate } from 'primeng/api';

import { Tenant } from '../../core/models/tenant';
import { TenantsService } from '../../core/tenants.service';
import { HistoryService } from '../../core/history.service';
import { EntitlementChange } from '../../core/models/history';
import { buildContinuousSeries } from '../../core/history-series';

type Mode = 'tenant' | 'portfolio';

/**
 * Time series with gaps as gaps (project brief §7.4). The chart is only
 * ever created once this component is actually on screen — never behind
 * a display:none tab — which is the mitigation for the hidden-canvas
 * pitfall called out in the brief: a canvas measured while hidden gets
 * width=0/height=0 and stays broken even after becoming visible. The
 * @if-gated chart below is destroyed and recreated on mode switch rather
 * than hidden, so it is never drawn without real dimensions.
 */
@Component({
    selector: 'app-history',
    imports: [FormsModule, TranslocoModule, ChartModule, SelectModule, SelectButtonModule, PrimeTemplate],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './history.component.html'
})
export class HistoryComponent implements OnInit {
  private readonly tenantsService = inject(TenantsService);
  private readonly historyService = inject(HistoryService);
  private readonly transloco = inject(TranslocoService);

  // Labels are rendered in the template via the `transloco` pipe (see the
  // item template on p-selectbutton) rather than computed here: a getter
  // returning a fresh array on every change-detection pass gave
  // p-selectbutton a new [options] reference each cycle, which triggered
  // its internal state update -> markForCheck -> another CD pass,
  // forever -- an infinite loop that hung the page. `labelKey` just carries
  // the translation key, a stable string, so [options] stays referentially
  // stable and CD settles normally.
  readonly modes: { labelKey: string; value: Mode }[] = [
    { labelKey: 'history.perTenant', value: 'tenant' },
    { labelKey: 'history.portfolio', value: 'portfolio' },
  ];
  readonly mode = signal<Mode>('tenant');

  readonly tenants = signal<Tenant[]>([]);
  readonly selectedTenant = signal<Tenant | null>(null);
  readonly loading = signal(false);
  readonly entitlementChanges = signal<EntitlementChange[]>([]);

  readonly chartData = signal<any | null>(null);
  readonly chartOptions = {
    plugins: { legend: { display: true } },
    scales: { y: { beginAtZero: true } },
    // spanGaps defaults to false in Chart.js -- explicit here because a
    // failed/missing collection day must render as a break, never an
    // interpolated line across it and never a drop to zero.
    spanGaps: false,
    elements: { line: { spanGaps: false } },
  };

  async ngOnInit(): Promise<void> {
    this.tenants.set(await this.tenantsService.list());
    if (this.tenants().length > 0) {
      this.selectedTenant.set(this.tenants()[0]);
      await this.loadTenantHistory();
    }
  }

  async setMode(mode: Mode): Promise<void> {
    this.mode.set(mode);
    this.chartData.set(null);
    if (mode === 'tenant' && this.selectedTenant()) {
      await this.loadTenantHistory();
    } else if (mode === 'portfolio') {
      await this.loadPortfolioHistory();
    }
  }

  async onTenantChange(): Promise<void> {
    await this.loadTenantHistory();
  }

  private async loadTenantHistory(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    this.loading.set(true);
    this.chartData.set(null);
    try {
      const history = await this.historyService.tenantHistory(tenant.id);
      this.entitlementChanges.set(history.entitlementChanges);

      const usedSeries = buildContinuousSeries(
        history.points.map((p) => ({ date: p.date, value: p.status === 'success' ? p.usedLicenses : null })),
      );
      const totalSeries = buildContinuousSeries(
        history.points.map((p) => ({ date: p.date, value: p.status === 'success' ? p.totalLicenses : null })),
      );

      this.chartData.set({
        labels: usedSeries.labels,
        datasets: [
          { label: this.transloco.translate('history.used'), data: usedSeries.values, borderColor: 'var(--p-primary-600)', backgroundColor: 'transparent', tension: 0.1 },
          { label: this.transloco.translate('history.entitled'), data: totalSeries.values, borderColor: 'var(--good-color)', backgroundColor: 'transparent', borderDash: [6, 3], tension: 0.1 },
        ],
      });
    } finally {
      this.loading.set(false);
    }
  }

  private async loadPortfolioHistory(): Promise<void> {
    this.loading.set(true);
    this.chartData.set(null);
    this.entitlementChanges.set([]);
    try {
      const points = await this.historyService.portfolioHistory();
      const usedSeries = buildContinuousSeries(points.map((p) => ({ date: p.date, value: p.totalUsed })));
      const totalSeries = buildContinuousSeries(points.map((p) => ({ date: p.date, value: p.totalEntitled })));

      this.chartData.set({
        labels: usedSeries.labels,
        datasets: [
          { label: this.transloco.translate('history.totalUsed'), data: usedSeries.values, borderColor: 'var(--p-primary-600)', backgroundColor: 'transparent', tension: 0.1 },
          { label: this.transloco.translate('history.totalEntitled'), data: totalSeries.values, borderColor: 'var(--good-color)', backgroundColor: 'transparent', borderDash: [6, 3], tension: 0.1 },
        ],
      });
    } finally {
      this.loading.set(false);
    }
  }
}
