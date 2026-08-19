import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { ChartModule } from 'primeng/chart';
import { CardModule } from 'primeng/card';
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
    imports: [FormsModule, TranslocoModule, ChartModule, CardModule, SelectModule, SelectButtonModule, PrimeTemplate],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './history.component.html',
    // PrimeNG's <p-chart> sets its own host [style] binding ("display:
    // block; position: relative;") with no height, which wins over any
    // plain style attribute passed to it in the template -- so its own
    // fixed height, and its wrapping height:34rem div above, can't reach
    // the canvas without going through the cascade instead. ::ng-deep
    // reliably overrides a child component's own host style binding,
    // which a template-level style/[style] binding does not.
    styles: [':host ::ng-deep p-chart { display: block; height: 100%; }']
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

  // True when the portfolio view's date range includes at least one day
  // where a tenant reported an unlimited license -- shown as a caption
  // under the chart, since TotalEntitled's sum is then a partial figure
  // (an unlimited tenant contributes 0, not its true, uncapped ceiling).
  readonly portfolioHasUnlimitedTenants = signal(false);

  readonly chartData = signal<any | null>(null);

  // Resolved once at construction, not read fresh per chart: this
  // screen has no theme toggle today (confirmed -- no ThemeService/
  // dark-mode anywhere in the frontend), so the tokens can't change
  // during a session. `getComputedStyle` is required here because
  // Chart.js draws on a <canvas> 2D context, which -- unlike CSS on a
  // DOM element -- cannot resolve a raw `var(--...)` string; passing one
  // as a canvas strokeStyle/fillStyle silently falls back to black,
  // which is exactly why this chart used to render with no color at all.
  private readonly root = getComputedStyle(document.documentElement);
  private readonly brand = this.resolveToken('--p-primary-600');
  private readonly capLine = this.resolveToken('--p-surface-400', '#a1a1aa');
  private readonly gridColor = this.resolveToken('--p-surface-200');
  private readonly axisColor = this.resolveToken('--p-surface-500');
  private readonly tooltipBg = this.resolveToken('--p-surface-0', '#ffffff');
  private readonly tooltipBorder = this.resolveToken('--frame-border-color', '#d4d4d8');
  private readonly fontFamily = this.resolveToken('--font-sans', 'Inter var, sans-serif');

  readonly chartOptions = {
    // Chart.js ignores a fixed-height CSS container unless told not to
    // preserve its own aspect ratio -- without this, it sizes the canvas
    // from its default 2:1 width/height ratio instead of the actual
    // 24rem container in history.component.html, so on a wide screen the
    // canvas renders taller than that box and overflows into (and, since
    // canvas backgrounds are transparent outside the plotted area, visibly
    // shows through) the Entitlement Changes list below it.
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: {
        display: true,
        labels: { usePointStyle: true, pointStyle: 'line', font: { family: this.fontFamily, size: 12 }, color: this.axisColor },
      },
      tooltip: {
        // 'nearest' snaps the tooltip to the closest actual data point.
        // The default 'average' positioner places it at the average
        // pixel height of every active series in the hovered column --
        // fine when the lines track close together, but once Entitled
        // is pinned flat at a high ceiling (an unlimited tenant) it lands
        // in the empty gap between the two lines instead of next to
        // either one.
        position: 'nearest' as const,
        backgroundColor: this.tooltipBg,
        borderColor: this.tooltipBorder,
        borderWidth: 1,
        titleColor: this.axisColor,
        bodyColor: this.axisColor,
        cornerRadius: 6,
        padding: 8,
        titleFont: { family: this.fontFamily, size: 11, weight: '600' as const },
        bodyFont: { family: this.fontFamily, size: 11 },
      },
    },
    scales: {
      x: { grid: { display: false }, ticks: { font: { family: this.fontFamily, size: 11 }, color: this.axisColor } },
      y: {
        beginAtZero: true,
        grid: { color: this.gridColor },
        ticks: { font: { family: this.fontFamily, size: 11 }, color: this.axisColor },
      },
    },
    // spanGaps defaults to false in Chart.js -- explicit here because a
    // failed/missing collection day must render as a break, never an
    // interpolated line across it and never a drop to zero.
    spanGaps: false,
    elements: {
      line: { spanGaps: false, tension: 0.3, borderWidth: 2.25 },
      point: { radius: 0, hoverRadius: 3 },
    },
  };

  /** Resolves a CSS custom property to its actual computed value (e.g.
   * "#0061a0"), which -- unlike the raw "var(--x)" string -- a canvas
   * 2D context can actually use as a stroke/fill color. Falls back to
   * fallback when the token isn't defined, so a renamed/missing token
   * degrades visibly (a fallback color) rather than invisibly (canvas's
   * own default-to-black behavior, the original bug this exists to
   * avoid repeating). */
  private resolveToken(varName: string, fallback = '#000000'): string {
    const value = this.root.getPropertyValue(varName).trim();
    return value || fallback;
  }

  /** Builds one chart's dataset pair (a "used" line with a brand-blue
   * fill, and a neutral dashed "entitled" ceiling line) from two
   * already-gap-filled series -- shared by loadTenantHistory and
   * loadPortfolioHistory, which previously duplicated this shape with
   * the exact same (broken) styling in both places. */
  private buildChartData(usedLabel: string, usedValues: (number | null)[], capLabel: string, capValues: (number | null)[], labels: string[]) {
    return {
      labels,
      datasets: [
        {
          label: usedLabel,
          data: usedValues,
          borderColor: this.brand,
          pointBackgroundColor: this.brand,
          fill: true,
          // A gradient reference to a specific canvas is created lazily
          // by Chart.js's own backgroundColor callback API -- this runs
          // once per render with the live chart context, so it always
          // matches the canvas's actual current size.
          backgroundColor: (context: any) => {
            const chart = context.chart;
            const { ctx, chartArea } = chart;
            if (!chartArea) {
              return 'transparent';
            }
            const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom);
            gradient.addColorStop(0, this.withAlpha(this.brand, 0.18));
            gradient.addColorStop(1, this.withAlpha(this.brand, 0.01));
            return gradient;
          },
        },
        {
          label: capLabel,
          data: capValues,
          borderColor: this.capLine,
          backgroundColor: 'transparent',
          borderDash: [7, 5],
          fill: false,
        },
      ],
    };
  }

  /** Turns a raw entitled/cap series -- where an unlimited day is a
   * successful collection with value 0 (ProfileUnity's own "no seat cap"
   * convention, see dashboard.IsUnlimitedLicense server-side) -- into one
   * that never plots that 0, per the confirmed design: the cap line
   * should stay flat at the highest known finite ceiling once a tenant
   * goes unlimited, not crater to zero. `null` (a genuine gap: the day
   * failed or was never attempted) is left untouched. */
  private buildEntitledSeries(values: (number | null)[]): (number | null)[] {
    const finiteValues = values.filter((v): v is number => v !== null && v > 0);
    const fallback = finiteValues.length > 0 ? Math.max(...finiteValues) : null;
    let lastFinite = fallback;
    return values.map((v) => {
      if (v === null) {
        return null;
      }
      if (v === 0) {
        return lastFinite;
      }
      lastFinite = v;
      return v;
    });
  }

  /** Adds an alpha channel to a "#rrggbb" color -- used to build the
   * "used" line's area-fill gradient without a second resolved token. */
  private withAlpha(hex: string, alpha: number): string {
    const match = /^#([0-9a-f]{6})$/i.exec(hex);
    if (!match) {
      return hex;
    }
    const int = parseInt(match[1], 16);
    const r = (int >> 16) & 255;
    const g = (int >> 8) & 255;
    const b = int & 255;
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }

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
    this.portfolioHasUnlimitedTenants.set(false);
    try {
      const history = await this.historyService.tenantHistory(tenant.id);
      this.entitlementChanges.set(history.entitlementChanges);

      const usedSeries = buildContinuousSeries(
        history.points.map((p) => ({ date: p.date, value: p.status === 'success' ? p.usedLicenses : null })),
      );
      const totalSeries = buildContinuousSeries(
        history.points.map((p) => ({ date: p.date, value: p.status === 'success' ? p.totalLicenses : null })),
      );
      const entitledValues = this.buildEntitledSeries(totalSeries.values);

      this.chartData.set(
        this.buildChartData(
          this.transloco.translate('history.used'),
          usedSeries.values,
          this.transloco.translate('history.entitled'),
          entitledValues,
          usedSeries.labels,
        ),
      );
    } finally {
      this.loading.set(false);
    }
  }

  private async loadPortfolioHistory(): Promise<void> {
    this.loading.set(true);
    this.chartData.set(null);
    this.entitlementChanges.set([]);
    this.portfolioHasUnlimitedTenants.set(false);
    try {
      const points = await this.historyService.portfolioHistory();
      this.portfolioHasUnlimitedTenants.set(points.some((p) => p.tenantsUnlimited > 0));
      const usedSeries = buildContinuousSeries(points.map((p) => ({ date: p.date, value: p.totalUsed })));
      const totalSeries = buildContinuousSeries(points.map((p) => ({ date: p.date, value: p.totalEntitled })));

      this.chartData.set(
        this.buildChartData(
          this.transloco.translate('history.totalUsed'),
          usedSeries.values,
          this.transloco.translate('history.totalEntitled'),
          totalSeries.values,
          usedSeries.labels,
        ),
      );
    } finally {
      this.loading.set(false);
    }
  }
}
