import { Component, OnInit, WritableSignal, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormBuilder, FormsModule, ReactiveFormsModule, Validators } from '@angular/forms';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { InputNumberModule } from 'primeng/inputnumber';
import { PasswordModule } from 'primeng/password';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { MessageModule } from 'primeng/message';

import { SettingsService } from '../../core/settings.service';
import { Settings } from '../../core/models/settings';

/** A short fallback list, only used if the browser doesn't support
 * Intl.supportedValuesOf('timeZone') (e.g. very old Safari/Firefox). */
const FALLBACK_TIMEZONES = [
  'UTC',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'Europe/London',
  'Europe/Amsterdam',
  'Europe/Berlin',
  'Europe/Paris',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Kolkata',
  'Australia/Sydney',
];

function buildTimezoneOptions(): string[] {
  const supportedValuesOf = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf;
  const zones = new Set(supportedValuesOf ? supportedValuesOf('timeZone') : FALLBACK_TIMEZONES);
  zones.add('UTC');
  return Array.from(zones).sort();
}

/**
 * Everything on this screen is optional to fill in at bootstrap time
 * (see PUMC_* in .env.example) and safe to change here afterward — SMTP/
 * report-email, collection tunables, session timeouts, and the active
 * TLS certificate. A save takes effect immediately, no restart, and is
 * reflected in the running scheduler/report-mail/session components and
 * (for a cert upload) the HTTPS listener itself.
 */
@Component({
    selector: 'app-settings',
    imports: [
        ReactiveFormsModule,
        FormsModule,
        TranslocoModule,
        ButtonModule,
        InputTextModule,
        InputNumberModule,
        PasswordModule,
        SelectModule,
        CardModule,
        MessageModule,
    ],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './settings.component.html',
})
export class SettingsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly settingsService = inject(SettingsService);

  readonly securityOptions = [
    { label: 'starttls', value: 'starttls' },
    { label: 'tls', value: 'tls' },
    { label: 'none', value: 'none' },
  ];

  /** Every IANA timezone name the browser's ICU data knows about (~400
   * entries), so the Timezone field is a real dropdown instead of a
   * free-text box an operator could mistype. Falls back to a short
   * curated list on a browser old enough not to support
   * Intl.supportedValuesOf. */
  readonly timezoneOptions: string[] = buildTimezoneOptions();

  /** Tracks the p-select's in-progress filter text so that, if the panel
   * closes (blur, click elsewhere, Save) while it uniquely narrows the
   * list to one zone, that zone is treated as selected — instead of
   * silently discarding what the operator typed because they never
   * explicitly clicked or Enter-selected the highlighted row. */
  private timezoneFilterQuery = '';

  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly saveError = signal<string | null>(null);
  readonly saveOk = signal(false);
  readonly current = signal<Settings | null>(null);

  readonly testingEmail = signal(false);
  readonly testEmailTo = signal('');
  readonly testEmailResult = signal<'ok' | 'error' | null>(null);

  readonly uploadingCert = signal(false);
  readonly certUploadError = signal<string | null>(null);
  readonly certPem = signal('');
  readonly keyPem = signal('');

  readonly form = this.fb.nonNullable.group({
    smtpHost: [''],
    smtpPort: [587, [Validators.min(1), Validators.max(65535)]],
    smtpUsername: [''],
    smtpPassword: [''],
    smtpFrom: [''],
    smtpSecurity: ['starttls' as 'starttls' | 'tls' | 'none'],
    reportRecipients: [''], // comma-separated; split on save
    reportEmailDay: [1, [Validators.min(1), Validators.max(28)]],
    collectionIntervalMinutes: [60, [Validators.min(1)]],
    collectionTimezone: ['UTC', Validators.required],
    collectionConcurrency: [5, [Validators.min(1)]],
    collectionTenantTimeoutSeconds: [30, [Validators.min(1)]],
    sessionIdleTimeoutMinutes: [30, [Validators.min(1)]],
    sessionAbsoluteTimeoutHours: [12, [Validators.min(1)]],
  });

  get existingSmtpPasswordSet(): boolean {
    return this.current()?.smtpPasswordSet ?? false;
  }

  onTimezoneFilter(event: { filter: string }): void {
    this.timezoneFilterQuery = event.filter ?? '';
  }

  onTimezoneHide(): void {
    const query = this.timezoneFilterQuery.trim().toLowerCase();
    this.timezoneFilterQuery = '';
    if (!query) {
      return;
    }
    const matches = this.timezoneOptions.filter((tz) => tz.toLowerCase().includes(query));
    if (matches.length === 1) {
      this.form.patchValue({ collectionTimezone: matches[0] });
    }
  }

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  private async reload(): Promise<void> {
    this.loading.set(true);
    try {
      const s = await this.settingsService.get();
      this.current.set(s);
      this.form.reset({
        smtpHost: s.smtpHost,
        smtpPort: s.smtpPort,
        smtpUsername: s.smtpUsername,
        smtpPassword: '',
        smtpFrom: s.smtpFrom,
        smtpSecurity: s.smtpSecurity,
        reportRecipients: s.reportRecipients.join(', '),
        reportEmailDay: s.reportEmailDay,
        collectionIntervalMinutes: Math.round(s.collectionIntervalSeconds / 60),
        collectionTimezone: s.collectionTimezone,
        collectionConcurrency: s.collectionConcurrency,
        collectionTenantTimeoutSeconds: s.collectionTenantTimeoutSeconds,
        sessionIdleTimeoutMinutes: Math.round(s.sessionIdleTimeoutSeconds / 60),
        sessionAbsoluteTimeoutHours: Math.round(s.sessionAbsoluteTimeoutSeconds / 3600),
      });
    } finally {
      this.loading.set(false);
    }
  }

  async save(): Promise<void> {
    if (this.form.invalid || this.saving()) {
      return;
    }
    this.saving.set(true);
    this.saveError.set(null);
    this.saveOk.set(false);
    const v = this.form.getRawValue();

    try {
      const updated = await this.settingsService.update({
        smtpHost: v.smtpHost,
        smtpPort: v.smtpPort,
        smtpUsername: v.smtpUsername,
        smtpPassword: v.smtpPassword ? v.smtpPassword : undefined,
        smtpFrom: v.smtpFrom,
        smtpSecurity: v.smtpSecurity,
        reportRecipients: v.reportRecipients
          .split(',')
          .map((r) => r.trim())
          .filter((r) => r.length > 0),
        reportEmailDay: v.reportEmailDay,
        collectionIntervalSeconds: v.collectionIntervalMinutes * 60,
        collectionTimezone: v.collectionTimezone,
        collectionConcurrency: v.collectionConcurrency,
        collectionTenantTimeoutSeconds: v.collectionTenantTimeoutSeconds,
        sessionIdleTimeoutSeconds: v.sessionIdleTimeoutMinutes * 60,
        sessionAbsoluteTimeoutSeconds: v.sessionAbsoluteTimeoutHours * 3600,
      });
      this.current.set(updated);
      this.form.patchValue({ smtpPassword: '' });
      this.saveOk.set(true);
    } catch {
      this.saveError.set('settings.saveError');
    } finally {
      this.saving.set(false);
    }
  }

  async sendTestEmail(): Promise<void> {
    if (!this.testEmailTo() || this.testingEmail()) {
      return;
    }
    this.testingEmail.set(true);
    this.testEmailResult.set(null);
    const v = this.form.getRawValue();
    try {
      await this.settingsService.testEmail({
        smtpHost: v.smtpHost,
        smtpPort: v.smtpPort,
        smtpUsername: v.smtpUsername,
        smtpPassword: v.smtpPassword ? v.smtpPassword : undefined,
        smtpFrom: v.smtpFrom,
        smtpSecurity: v.smtpSecurity,
        to: this.testEmailTo(),
      });
      this.testEmailResult.set('ok');
    } catch {
      this.testEmailResult.set('error');
    } finally {
      this.testingEmail.set(false);
    }
  }

  onCertFileSelected(event: Event): void {
    this.readFileIntoSignal(event, this.certPem);
  }

  onKeyFileSelected(event: Event): void {
    this.readFileIntoSignal(event, this.keyPem);
  }

  private readFileIntoSignal(event: Event, target: WritableSignal<string>): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    const reader = new FileReader();
    reader.onload = () => target.set(String(reader.result ?? ''));
    reader.readAsText(file);
  }

  async uploadCert(): Promise<void> {
    if (!this.certPem() || !this.keyPem() || this.uploadingCert()) {
      return;
    }
    this.uploadingCert.set(true);
    this.certUploadError.set(null);
    try {
      const updated = await this.settingsService.uploadTlsCert({ certPem: this.certPem(), keyPem: this.keyPem() });
      this.current.set(updated);
      this.certPem.set('');
      this.keyPem.set('');
    } catch {
      this.certUploadError.set('settings.tlsCertUploadError');
    } finally {
      this.uploadingCert.set(false);
    }
  }
}
