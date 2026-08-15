import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

import { Alert } from './models/alert';

@Injectable({ providedIn: 'root' })
export class AlertsService {
  private readonly http = inject(HttpClient);

  list(): Promise<Alert[]> {
    return firstValueFrom(this.http.get<Alert[]>('/api/alerts'));
  }
}
