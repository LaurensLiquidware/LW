import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, FormsModule, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { InputNumberModule } from 'primeng/inputnumber';
import { PasswordModule } from 'primeng/password';
import { CheckboxModule } from 'primeng/checkbox';
import { MessageModule } from 'primeng/message';
import { TableModule } from 'primeng/table';
import { ConfirmationService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';

import { Tenant } from '../../core/models/tenant';
import { TenantsService } from '../../core/tenants.service';
import { LicensesService } from '../../core/licenses.service';
import { LicenseServerConnection, DecodedLicense, LicensePushResult, LicensePushRecord } from '../../core/models/license';

/**
 * Pushes a signed license to a tenant's ProfileUnity License Server -- a
 * distinct host/credential from the tenant's own console (see
 * core/models/license.ts). Three independent sections per selected
 * tenant: the connection itself, the push action (decode-then-confirm),
 * and this tenant's push history -- the License Server itself keeps no
 * record of what it replaced.
 */
@Component({
    selector: 'app-licenses',
    imports: [
        FormsModule,
        ReactiveFormsModule,
        TranslocoModule,
        SelectModule,
        CardModule,
        ButtonModule,
        InputTextModule,
        InputNumberModule,
        PasswordModule,
        CheckboxModule,
        MessageModule,
        TableModule,
        ConfirmDialogModule,
    ],
    providers: [ConfirmationService],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './licenses.component.html',
})
export class LicensesComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly tenantsService = inject(TenantsService);
  private readonly licenses = inject(LicensesService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly transloco = inject(TranslocoService);

  readonly tenants = signal<Tenant[]>([]);
  readonly selectedTenant = signal<Tenant | null>(null);

  readonly connection = signal<LicenseServerConnection | null>(null);
  readonly loadingConnection = signal(false);
  readonly savingConnection = signal(false);
  readonly saveError = signal<string | null>(null);
  readonly checkingUp = signal(false);
  readonly checkupResult = signal<{ ok: boolean; message: string } | null>(null);

  readonly connectionForm = this.fb.nonNullable.group({
    hostname: ['', Validators.required],
    port: [443, [Validators.required, Validators.min(1), Validators.max(65535)]],
    username: [''],
    password: [''],
    tlsSkipVerify: [false],
  });

  readonly licenseInput = signal('');
  readonly previewResult = signal<DecodedLicense | null>(null);
  readonly previewError = signal<string | null>(null);
  readonly previewing = signal(false);
  readonly pushing = signal(false);
  readonly pushResult = signal<LicensePushResult | null>(null);

  readonly history = signal<LicensePushRecord[]>([]);
  readonly loadingHistory = signal(false);

  get existingHasPassword(): boolean {
    return this.connection()?.hasPassword ?? false;
  }

  async ngOnInit(): Promise<void> {
    this.tenants.set(await this.tenantsService.list());
    if (this.tenants().length > 0) {
      this.selectedTenant.set(this.tenants()[0]);
      await this.onTenantChange();
    }
  }

  async onTenantChange(): Promise<void> {
    this.checkupResult.set(null);
    this.saveError.set(null);
    this.licenseInput.set('');
    this.previewResult.set(null);
    this.previewError.set(null);
    this.pushResult.set(null);
    await Promise.all([this.loadConnection(), this.loadHistory()]);
  }

  private async loadConnection(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    this.loadingConnection.set(true);
    try {
      const conn = await this.licenses.getConnection(tenant.id);
      this.connection.set(conn);
      if (conn.hostname) {
        // Already configured -- show exactly what's stored, no prefill.
        this.connectionForm.reset({
          hostname: conn.hostname,
          port: conn.port,
          username: conn.username,
          password: '',
          tlsSkipVerify: conn.tlsSkipVerify,
        });
      } else {
        // Nothing configured yet -- suggest this tenant's own console
        // hostname (License Servers are commonly on the same or a
        // near-identical host) and the conventional service-account
        // username. Both are editable defaults only, saved nowhere
        // until the operator clicks Save.
        this.connectionForm.reset({
          hostname: tenant.hostname,
          port: 443,
          username: 'prou_services',
          password: '',
          tlsSkipVerify: false,
        });
      }
    } finally {
      this.loadingConnection.set(false);
    }
  }

  async saveConnection(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant || this.connectionForm.invalid) {
      return;
    }
    this.savingConnection.set(true);
    this.saveError.set(null);
    try {
      const v = this.connectionForm.getRawValue();
      let password: string | undefined = v.password || undefined;
      if (!v.password) {
        password = v.username ? undefined : '';
      }
      const conn = await this.licenses.saveConnection(tenant.id, {
        hostname: v.hostname,
        port: v.port,
        username: v.username,
        password,
        tlsSkipVerify: v.tlsSkipVerify,
      });
      this.connection.set(conn);
      this.connectionForm.patchValue({ password: '' });
    } catch {
      this.saveError.set('licenses.saveError');
    } finally {
      this.savingConnection.set(false);
    }
  }

  async checkup(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    const v = this.connectionForm.getRawValue();
    this.checkingUp.set(true);
    this.checkupResult.set(null);
    try {
      this.checkupResult.set(await this.licenses.checkup(tenant.id, { hostname: v.hostname, port: v.port, tlsSkipVerify: v.tlsSkipVerify }));
    } catch {
      this.checkupResult.set({ ok: false, message: this.transloco.translate('licenses.checkupError') });
    } finally {
      this.checkingUp.set(false);
    }
  }

  async previewLicense(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant || !this.licenseInput().trim()) {
      return;
    }
    this.previewing.set(true);
    this.previewError.set(null);
    this.previewResult.set(null);
    this.pushResult.set(null);
    try {
      this.previewResult.set(await this.licenses.preview(tenant.id, this.licenseInput().trim()));
    } catch {
      this.previewError.set('licenses.previewError');
    } finally {
      this.previewing.set(false);
    }
  }

  confirmPush(): void {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    this.confirmation.confirm({
      header: this.transloco.translate('licenses.pushConfirmTitle'),
      message: this.transloco.translate('licenses.pushConfirmMessage', { hostname: this.connection()?.hostname ?? '' }),
      acceptLabel: this.transloco.translate('licenses.push'),
      rejectLabel: this.transloco.translate('licenses.cancel'),
      accept: () => this.push(),
    });
  }

  private async push(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    this.pushing.set(true);
    this.pushResult.set(null);
    try {
      const result = await this.licenses.push(tenant.id, this.licenseInput().trim(), true);
      this.pushResult.set(result);
      await this.loadHistory();
    } catch {
      this.pushResult.set({ outcome: 'error', message: this.transloco.translate('licenses.pushError'), fields: this.previewResult()! });
    } finally {
      this.pushing.set(false);
    }
  }

  private async loadHistory(): Promise<void> {
    const tenant = this.selectedTenant();
    if (!tenant) {
      return;
    }
    this.loadingHistory.set(true);
    try {
      this.history.set(await this.licenses.history(tenant.id));
    } finally {
      this.loadingHistory.set(false);
    }
  }
}
