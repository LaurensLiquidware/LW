import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

/** Reads the server's version from the public /api/version endpoint --
 * used by both the About screen and the header badge, so this is the one
 * place that knows the response shape. */
@Injectable({ providedIn: 'root' })
export class VersionService {
  private readonly http = inject(HttpClient);

  fetch(): Promise<string> {
    return firstValueFrom(this.http.get<{ version: string }>('/api/version')).then((res) => res.version);
  }
}
