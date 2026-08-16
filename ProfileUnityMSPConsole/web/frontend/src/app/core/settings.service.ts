import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { Settings, SettingsWriteRequest, TlsCertUploadRequest, LogoUploadRequest, TestEmailRequest } from './models/settings';

@Injectable({ providedIn: 'root' })
export class SettingsService {
  private readonly http = inject(HttpClient);

  get(): Promise<Settings> {
    return firstValueFrom(this.http.get<Settings>('/api/settings'));
  }

  update(req: SettingsWriteRequest): Promise<Settings> {
    return firstValueFrom(this.http.put<Settings>('/api/settings', req));
  }

  uploadTlsCert(req: TlsCertUploadRequest): Promise<Settings> {
    return firstValueFrom(this.http.post<Settings>('/api/settings/tls-cert', req));
  }

  uploadLogo(req: LogoUploadRequest): Promise<Settings> {
    return firstValueFrom(this.http.post<Settings>('/api/settings/logo', req));
  }

  clearLogo(): Promise<Settings> {
    return firstValueFrom(this.http.delete<Settings>('/api/settings/logo'));
  }

  /** Cache-busting URL for the Settings screen's logo <img> preview --
   * the browser must not reuse a stale cached image after a fresh
   * upload/clear. */
  logoPreviewUrl(): string {
    return `/api/settings/logo?t=${Date.now()}`;
  }

  /** Sends a one-off test message using form values, not the saved
   * settings, so an operator can confirm SMTP works before saving it. */
  testEmail(req: TestEmailRequest): Promise<void> {
    return firstValueFrom(this.http.post<void>('/api/settings/test-email', req));
  }

  /** Sends last calendar month's portfolio report by email immediately,
   * using the saved settings, bypassing the scheduled send day. */
  sendReportNow(): Promise<{ year: number; month: number }> {
    return firstValueFrom(this.http.post<{ year: number; month: number }>('/api/settings/send-report-now', {}));
  }
}
