import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { Settings, SettingsWriteRequest, TlsCertUploadRequest, TestEmailRequest } from './models/settings';

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

  /** Sends a one-off test message using form values, not the saved
   * settings, so an operator can confirm SMTP works before saving it. */
  testEmail(req: TestEmailRequest): Promise<void> {
    return firstValueFrom(this.http.post<void>('/api/settings/test-email', req));
  }
}
