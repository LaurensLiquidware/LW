import { Component, EventEmitter, Input, OnChanges, Output, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { InputNumberModule } from 'primeng/inputnumber';
import { PasswordModule } from 'primeng/password';
import { CheckboxModule } from 'primeng/checkbox';
import { MessageModule } from 'primeng/message';

import { Tenant, TenantWriteRequest, TestConnectionResponse } from '../../core/models/tenant';
import { TenantsService } from '../../core/tenants.service';

/**
 * Create-and-edit form for one tenant registration (project brief §7.1).
 * Password uses the same three-way semantics as the backend: left blank
 * on an edit, the stored password is untouched; typed, it replaces it.
 * There is no way to view a stored password, ever — only whether one
 * exists (existingHasPassword).
 */
@Component({
  selector: 'app-tenant-form',
  standalone: true,
  imports: [
    ReactiveFormsModule,
    TranslocoModule,
    ButtonModule,
    InputTextModule,
    InputNumberModule,
    PasswordModule,
    CheckboxModule,
    MessageModule,
  ],
  templateUrl: './tenant-form.component.html',
})
export class TenantFormComponent implements OnChanges {
  private readonly fb = inject(FormBuilder);
  private readonly tenants = inject(TenantsService);

  @Input() tenant: Tenant | null = null;
  @Output() saved = new EventEmitter<void>();
  @Output() cancelled = new EventEmitter<void>();

  readonly form = this.fb.nonNullable.group({
    displayName: ['', Validators.required],
    hostname: ['', Validators.required],
    port: [8000, [Validators.required, Validators.min(1), Validators.max(65535)]],
    username: [''],
    password: [''],
    tlsSkipVerify: [false],
    enabled: [true],
    tags: [''], // comma-separated; split on save
    notes: [''],
  });

  readonly saving = signal(false);
  readonly saveError = signal<string | null>(null);
  readonly testing = signal(false);
  readonly testResult = signal<TestConnectionResponse | null>(null);

  get existingHasPassword(): boolean {
    return this.tenant?.hasPassword ?? false;
  }

  get isEdit(): boolean {
    return this.tenant !== null;
  }

  ngOnChanges(): void {
    this.testResult.set(null);
    this.saveError.set(null);
    if (this.tenant) {
      this.form.reset({
        displayName: this.tenant.displayName,
        hostname: this.tenant.hostname,
        port: this.tenant.port,
        username: this.tenant.username,
        password: '',
        tlsSkipVerify: this.tenant.tlsSkipVerify,
        enabled: this.tenant.enabled,
        tags: this.tenant.tags.join(', '),
        notes: this.tenant.notes,
      });
    } else {
      this.form.reset({ displayName: '', hostname: '', port: 8000, username: '', password: '', tlsSkipVerify: false, enabled: true, tags: '', notes: '' });
    }
  }

  async testConnection(): Promise<void> {
    const v = this.form.getRawValue();
    this.testing.set(true);
    this.testResult.set(null);
    try {
      const result = await this.tenants.testConnection({
        hostname: v.hostname,
        port: v.port,
        tlsSkipVerify: v.tlsSkipVerify,
        username: v.username,
        password: v.password || null,
      });
      this.testResult.set(result);
    } finally {
      this.testing.set(false);
    }
  }

  async save(): Promise<void> {
    if (this.form.invalid || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.saveError.set(null);
    const v = this.form.getRawValue();

    // password: undefined (unchanged) if editing and left blank; null
    // (clear) if editing and username was cleared; the typed value
    // otherwise. On create, an empty string is fine — CreateInput treats
    // "" as "no credentials".
    let password: string | null | undefined = v.password || undefined;
    if (this.isEdit && !v.password) {
      password = v.username ? undefined : null;
    }

    const req: TenantWriteRequest = {
      displayName: v.displayName,
      hostname: v.hostname,
      port: v.port,
      username: v.username,
      password,
      tlsSkipVerify: v.tlsSkipVerify,
      enabled: v.enabled,
      tags: v.tags
        .split(',')
        .map((tag) => tag.trim())
        .filter((tag) => tag.length > 0),
      notes: v.notes,
    };

    try {
      if (this.tenant) {
        await this.tenants.update(this.tenant.id, req);
      } else {
        await this.tenants.create(req);
      }
      this.saved.emit();
    } catch {
      this.saveError.set('tenants.saveError');
    } finally {
      this.saving.set(false);
    }
  }
}
