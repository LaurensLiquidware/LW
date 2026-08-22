import { Injectable, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

export interface PickOptions {
  title?: string;
  /** Win32 FileDialog filter string, e.g. "FlexApp Packages
   * (*.vhdx;*.zip)|*.vhdx;*.zip|All Files (*.*)|*.*". Ignored by
   * pickFolder. */
  filter?: string;
}

/** Talks to the native file/folder picker the server itself exposes on
 * Windows (GET /api/pick-file, GET /api/pick-folder -- see
 * internal/httpapi/picker_windows.go). Same-origin, no separate address
 * or health probe needed: GET /api/config reports whether this build of
 * the server was compiled with picker support at all (Windows only), and
 * the New Scan screen only renders its Browse buttons when it is. */
@Injectable({ providedIn: 'root' })
export class PickerService {
  private readonly http = inject(HttpClient);
  private checkPromise: Promise<boolean> | null = null;

  /** Whether the picker is available on this server build. Starts
   * false; becomes accurate once checkAvailable() resolves. */
  readonly available = signal(false);

  /** Checks GET /api/config at most once per page load and caches the
   * result; safe to call from multiple components. */
  checkAvailable(): Promise<boolean> {
    if (!this.checkPromise) {
      this.checkPromise = firstValueFrom(this.http.get<{ pickerAvailable: boolean }>('/api/config'))
        .then((cfg) => {
          const ok = !!cfg.pickerAvailable;
          this.available.set(ok);
          return ok;
        })
        .catch(() => {
          this.available.set(false);
          return false;
        });
    }
    return this.checkPromise;
  }

  /** Shows a native "Open File" dialog and resolves with the chosen
   * path, or null if the dialog was canceled or the picker isn't
   * available. */
  pickFile(options?: PickOptions): Promise<string | null> {
    return this.pick('/api/pick-file', options);
  }

  /** Shows a native "Browse for Folder" dialog and resolves with the
   * chosen path, or null if the dialog was canceled or the picker isn't
   * available. */
  pickFolder(options?: PickOptions): Promise<string | null> {
    return this.pick('/api/pick-folder', options);
  }

  private async pick(path: string, options?: PickOptions): Promise<string | null> {
    const params: Record<string, string> = {};
    if (options?.title) {
      params['title'] = options.title;
    }
    if (options?.filter) {
      params['filter'] = options.filter;
    }
    try {
      // A canceled dialog answers 204 No Content, which HttpClient
      // resolves as a null body rather than an error.
      const res = await firstValueFrom(this.http.get<{ path: string } | null>(path, { params }));
      return res?.path || null;
    } catch {
      return null;
    }
  }
}
