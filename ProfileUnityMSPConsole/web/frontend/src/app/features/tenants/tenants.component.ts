import { Component, OnInit, inject, signal } from '@angular/core';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';
import { TagModule } from 'primeng/tag';
import { ConfirmationService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';

import { Tenant } from '../../core/models/tenant';
import { TenantsService } from '../../core/tenants.service';
import { TenantFormComponent } from './tenant-form.component';

@Component({
  selector: 'app-tenants',
  standalone: true,
  imports: [TranslocoModule, TableModule, ButtonModule, DialogModule, TagModule, ConfirmDialogModule, TenantFormComponent],
  providers: [ConfirmationService],
  templateUrl: './tenants.component.html',
})
export class TenantsComponent implements OnInit {
  private readonly tenants = inject(TenantsService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly transloco = inject(TranslocoService);

  readonly rows = signal<Tenant[]>([]);
  readonly loading = signal(true);
  readonly dialogVisible = signal(false);
  readonly editing = signal<Tenant | null>(null);

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  async reload(): Promise<void> {
    this.loading.set(true);
    try {
      this.rows.set(await this.tenants.list());
    } finally {
      this.loading.set(false);
    }
  }

  openCreate(): void {
    this.editing.set(null);
    this.dialogVisible.set(true);
  }

  openEdit(tenant: Tenant): void {
    this.editing.set(tenant);
    this.dialogVisible.set(true);
  }

  async onSaved(): Promise<void> {
    this.dialogVisible.set(false);
    await this.reload();
  }

  confirmDelete(tenant: Tenant): void {
    this.confirmation.confirm({
      header: this.transloco.translate('tenants.deleteConfirmTitle'),
      message: this.transloco.translate('tenants.deleteConfirmMessage', { displayName: tenant.displayName }),
      acceptLabel: this.transloco.translate('tenants.delete'),
      rejectLabel: this.transloco.translate('tenants.cancel'),
      accept: async () => {
        await this.tenants.delete(tenant.id);
        await this.reload();
      },
    });
  }
}
