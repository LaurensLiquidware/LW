import { Component, OnInit, ViewChild, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { TranslocoModule } from '@jsverse/transloco';
import { TableModule, Table } from 'primeng/table';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';
import { MessageModule } from 'primeng/message';

import { TenantStatus } from '../../core/models/dashboard';
import { TenantsService } from '../../core/tenants.service';
import { SettingsService } from '../../core/settings.service';
import { StatusBadgeComponent } from '../../shared/status-badge.component';

/**
 * All tenants at a glance (project brief §7.3). Sorting/filtering is
 * PrimeNG Table's built-in per-column sort plus a global text filter —
 * no bespoke sort/filter logic to keep in sync with the table markup.
 */
@Component({
    selector: 'app-dashboard',
    imports: [TranslocoModule, TableModule, InputTextModule, ButtonModule, MessageModule, StatusBadgeComponent, DecimalPipe],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './dashboard.component.html'
})
export class DashboardComponent implements OnInit {
  private readonly tenants = inject(TenantsService);
  private readonly settings = inject(SettingsService);
  @ViewChild('table') table?: Table;

  readonly rows = signal<TenantStatus[]>([]);
  readonly loading = signal(true);
  readonly collecting = signal(false);
  readonly collectError = signal<string | null>(null);

  /** The MSP operator's own company name/logo (Settings > Company
   * Branding), shown alongside the page title -- same branding that
   * already appears on every PDF report. */
  readonly companyName = signal<string | null>(null);
  readonly logoUrl = signal<string | null>(null);

  async ngOnInit(): Promise<void> {
    await Promise.all([this.reload(), this.loadBranding()]);
  }

  private async loadBranding(): Promise<void> {
    try {
      const s = await this.settings.get();
      this.companyName.set(s.companyName || null);
      this.logoUrl.set(s.companyLogoConfigured ? this.settings.logoPreviewUrl() : null);
    } catch {
      // Branding is decorative here -- a failed fetch just means the
      // page shows no company name/logo, not a broken dashboard.
    }
  }

  async reload(): Promise<void> {
    this.loading.set(true);
    try {
      this.rows.set(await this.tenants.dashboard());
    } finally {
      this.loading.set(false);
    }
  }

  /** Manual "Collect Now" (project brief §7.2) — runs the same collection
   * pass the scheduler's ticker runs, so a newly-added tenant doesn't
   * have to wait for the next scheduled interval, then refreshes the
   * table against the new snapshots. */
  async collectNow(): Promise<void> {
    this.collecting.set(true);
    this.collectError.set(null);
    try {
      await this.tenants.collectNow();
      await this.reload();
    } catch {
      this.collectError.set('dashboard.collectNowError');
    } finally {
      this.collecting.set(false);
    }
  }

  applyGlobalFilter(value: string): void {
    this.table?.filterGlobal(value, 'contains');
  }
}
