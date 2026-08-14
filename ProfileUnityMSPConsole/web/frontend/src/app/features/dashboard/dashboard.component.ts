import { Component, OnInit, ViewChild, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { DecimalPipe } from '@angular/common';
import { TranslocoModule } from '@jsverse/transloco';
import { TableModule, Table } from 'primeng/table';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';

import { TenantStatus } from '../../core/models/dashboard';
import { TenantsService } from '../../core/tenants.service';
import { StatusBadgeComponent } from '../../shared/status-badge.component';

/**
 * All tenants at a glance (project brief §7.3). Sorting/filtering is
 * PrimeNG Table's built-in per-column sort plus a global text filter —
 * no bespoke sort/filter logic to keep in sync with the table markup.
 */
@Component({
    selector: 'app-dashboard',
    imports: [TranslocoModule, TableModule, InputTextModule, ButtonModule, StatusBadgeComponent, DecimalPipe],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './dashboard.component.html'
})
export class DashboardComponent implements OnInit {
  private readonly tenants = inject(TenantsService);
  @ViewChild('table') table?: Table;

  readonly rows = signal<TenantStatus[]>([]);
  readonly loading = signal(true);

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  async reload(): Promise<void> {
    this.loading.set(true);
    try {
      this.rows.set(await this.tenants.dashboard());
    } finally {
      this.loading.set(false);
    }
  }

  applyGlobalFilter(value: string): void {
    this.table?.filterGlobal(value, 'contains');
  }
}
