import { Injectable, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';

/** How long to wait for the picker's /health check before giving up and
 * treating it as unreachable -- generous enough for a same-machine
 * loopback call, short enough that a New Scan screen with no tray picker
 * running (this Linux dev environment, or the server run directly
 * without the tray launcher) doesn't visibly stall before hiding its
 * Browse buttons. */
const PROBE_TIMEOUT_MS = 800;

export interface PickOptions {
  title?: string;
  /** Win32 FileDialog filter string, e.g. "FlexApp Packages
   * (*.vhdx;*.zip)|*.vhdx;*.zip|All Files (*.*)|*.*" -- passed through
   * verbatim to the tray's picker server. Ignored by pickFolder. */
  filter?: string;
}

/** Talks to the native file/folder picker cmd/tray hosts on Windows (see
 * cmd/tray/picker_windows.go), whose address the server reports via
 * GET /api/config. The picker is optional: it only exists when this app
 * was launched via the tray launcher on Windows, so every method here
 * degrades to "unavailable" rather than throwing when it's unreachable
 * (not running via tray, or on a non-Windows dev machine). */
@Injectable({ providedIn: 'root' })
export class PickerService {
  private readonly http = inject(HttpClient);
  private pickerAddr: string | null = null;
  private probePromise: Promise<boolean> | null = null;

  /** Whether the picker was reachable the last time it was probed.
   * Starts false; becomes accurate once checkAvailable() resolves. */
  readonly available = signal(false);

  /** Probes the picker's reachability at most once per page load and
   * caches the result; safe to call from multiple components. */
  checkAvailable(): Promise<boolean> {
    if (!this.probePromise) {
      this.probePromise = this.probe();
    }
    return this.probePromise;
  }

  private async probe(): Promise<boolean> {
    try {
      const cfg = await firstValueFrom(this.http.get<{ pickerAddr: string }>('/api/config'));
      if (!cfg.pickerAddr) {
        this.available.set(false);
        return false;
      }
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), PROBE_TIMEOUT_MS);
      try {
        const res = await fetch(`http://${cfg.pickerAddr}/health`, { signal: controller.signal });
        if (res.ok) {
          this.pickerAddr = cfg.pickerAddr;
        }
        this.available.set(res.ok);
        return res.ok;
      } finally {
        clearTimeout(timer);
      }
    } catch {
      this.available.set(false);
      return false;
    }
  }

  /** Shows a native "Open File" dialog and resolves with the chosen
   * path, or null if the dialog was canceled or the picker isn't
   * available. */
  pickFile(options?: PickOptions): Promise<string | null> {
    return this.pick('pick-file', options);
  }

  /** Shows a native "Browse for Folder" dialog and resolves with the
   * chosen path, or null if the dialog was canceled or the picker isn't
   * available. */
  pickFolder(options?: PickOptions): Promise<string | null> {
    return this.pick('pick-folder', options);
  }

  private async pick(endpoint: 'pick-file' | 'pick-folder', options?: PickOptions): Promise<string | null> {
    if (!this.pickerAddr) {
      return null;
    }
    const params = new URLSearchParams();
    if (options?.title) {
      params.set('title', options.title);
    }
    if (options?.filter) {
      params.set('filter', options.filter);
    }
    const qs = params.toString();
    const url = `http://${this.pickerAddr}/${endpoint}${qs ? `?${qs}` : ''}`;

    let res: Response;
    try {
      res = await fetch(url);
    } catch {
      return null;
    }
    if (res.status === 204 || !res.ok) {
      return null;
    }
    const body = (await res.json()) as { path: string };
    return body.path || null;
  }
}
