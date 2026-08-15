import { Component, OnInit, WritableSignal, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { AbstractControl, FormBuilder, FormsModule, ReactiveFormsModule, ValidationErrors, ValidatorFn, Validators } from '@angular/forms';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { InputNumberModule } from 'primeng/inputnumber';
import { PasswordModule } from 'primeng/password';
import { SelectModule } from 'primeng/select';
import { CardModule } from 'primeng/card';
import { MessageModule } from 'primeng/message';
import { ConfirmationService } from 'primeng/api';
import { ConfirmDialogModule } from 'primeng/confirmdialog';

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

/** Mirrors internal/settings.Settings.Validate's SMTP cross-field rule
 * exactly, so an operator sees why Save is disabled before submitting
 * instead of getting a 400 back after the fact: once an SMTP host is
 * set, a From address and at least one report recipient become
 * required; with no host, neither may be set either. */
function smtpCrossFieldValidator(): ValidatorFn {
  return (group: AbstractControl): ValidationErrors | null => {
    const host: string = group.get('smtpHost')?.value ?? '';
    const from: string = group.get('smtpFrom')?.value ?? '';
    const recipients: string = group.get('reportRecipients')?.value ?? '';
    if (host !== '') {
      if (from === '') {
        return { smtpFromRequired: true };
      }
      if (recipients.trim() === '') {
        return { reportRecipientsRequired: true };
      }
    } else if (from !== '' || recipients.trim() !== '') {
      return { smtpHostRequired: true };
    }
    return null;
  };
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
        ConfirmDialogModule,
    ],
    providers: [ConfirmationService],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './settings.component.html',
})
export class SettingsComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly settingsService = inject(SettingsService);
  private readonly confirmation = inject(ConfirmationService);
  private readonly transloco = inject(TranslocoService);

  readonly securityOptions = [
    { label: 'starttls', value: 'starttls' },
    { label: 'tls', value: 'tls' },
    { label: 'none', value: 'none' },
  ];

  /** The conventional port for each security mode -- selecting a
   * security mode patches the port to match, since that's what almost
   * every SMTP relay actually expects; the port field stays editable
   * afterward for the rare relay that uses something else. */
  private readonly defaultPortForSecurity: Record<'starttls' | 'tls' | 'none', number> = {
    starttls: 587,
    tls: 465,
    none: 25,
  };

  private static readonly STANDARD_SMTP_PORTS = [25, 465, 587];

  /** The dropdown's option list -- normally just the three standard
   * ports, but widened to also include whatever port is currently
   * loaded/typed if it's a non-standard one, so an existing custom
   * port never silently vanishes from the list. */
  readonly smtpPortOptions = signal(SettingsComponent.STANDARD_SMTP_PORTS.map((port) => ({ label: String(port), value: port })));

  private ensureSmtpPortOption(port: number): void {
    if (SettingsComponent.STANDARD_SMTP_PORTS.includes(port)) {
      return;
    }
    this.smtpPortOptions.update((options) =>
      options.some((o) => o.value === port)
        ? options
        : [...options, { label: String(port), value: port }].sort((a, b) => a.value - b.value),
    );
  }

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
  /** Whether saveError holds a raw backend message (already in English,
   * shown as-is) rather than a transloco key. */
  readonly saveErrorIsRaw = signal(false);
  readonly saveOk = signal(false);
  readonly current = signal<Settings | null>(null);

  readonly testingEmail = signal(false);
  readonly testEmailTo = signal('');
  readonly testEmailResult = signal<'ok' | 'error' | null>(null);

  readonly sendingReportNow = signal(false);
  readonly sendReportNowResult = signal<'ok' | 'error' | null>(null);

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
  }, { validators: smtpCrossFieldValidator() });

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

  /** Fires only on the operator actually picking a security mode (not on
   * a programmatic reset/patch, since PrimeNG's onChange only reflects
   * real user interaction) -- sets the port to that mode's conventional
   * default. The port field stays a normal control afterward, so it can
   * still be overridden for a relay that uses a non-standard port. */
  onSecurityChange(event: { value: 'starttls' | 'tls' | 'none' }): void {
    this.form.patchValue({ smtpPort: this.defaultPortForSecurity[event.value] });
  }

  async ngOnInit(): Promise<void> {
    await this.reload();
  }

  private async reload(): Promise<void> {
    this.loading.set(true);
    try {
      const s = await this.settingsService.get();
      this.current.set(s);
      this.ensureSmtpPortOption(s.smtpPort);
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
    } catch (err) {
      // A 400 here is Settings.Validate() rejecting the payload (see
      // internal/settings/settings.go) -- its message says exactly what's
      // wrong (e.g. a missing From address), so show it verbatim instead
      // of a generic "could not save" that leaves the operator guessing.
      const detail = err instanceof HttpErrorResponse && err.status === 400 && typeof err.error === 'string' ? err.error.trim() : '';
      if (detail) {
        this.saveError.set(detail);
        this.saveErrorIsRaw.set(true);
      } else {
        this.saveError.set('settings.saveError');
        this.saveErrorIsRaw.set(false);
      }
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

  /** Confirms before sending, since unlike Test Email (which only ever
   * goes to an address the operator just typed in) this emails every
   * configured report recipient immediately -- the same treatment this
   * codebase gives tenant deletion, the one other action here with a
   * real effect outside this screen. */
  confirmSendReportNow(): void {
    this.confirmation.confirm({
      header: this.transloco.translate('settings.sendReportNowConfirmTitle'),
      message: this.transloco.translate('settings.sendReportNowConfirmMessage'),
      acceptLabel: this.transloco.translate('settings.sendReportNow'),
      rejectLabel: this.transloco.translate('settings.cancel'),
      accept: () => this.sendReportNow(),
    });
  }

  private async sendReportNow(): Promise<void> {
    if (this.sendingReportNow()) {
      return;
    }
    this.sendingReportNow.set(true);
    this.sendReportNowResult.set(null);
    try {
      await this.settingsService.sendReportNow();
      this.sendReportNowResult.set('ok');
    } catch {
      this.sendReportNowResult.set('error');
    } finally {
      this.sendingReportNow.set(false);
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
