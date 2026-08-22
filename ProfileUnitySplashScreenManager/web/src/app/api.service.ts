import { Injectable } from '@angular/core';
import { AboutInfo, AppState } from './models';

/**
 * Handshake the Go host injects into the page before any script runs, via the
 * WebView's initialisation script. Keeping the token out of the URL keeps it out
 * of history, referrers and logs.
 */
interface HostHandshake {
  token: string;
  origin: string;
  version: string;
  placeholderUI: boolean;
}

declare global {
  interface Window {
    __PSM__?: HostHandshake;
  }
}

/** Raised for an error the API reported in its JSON body. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly host = window.__PSM__;

  /** True when the page is not running inside the Go host. */
  readonly detached = !this.host?.token;

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await this.fetchRaw(path, init);
    if (response.status === 204) {
      return undefined as T;
    }
    return (await response.json()) as T;
  }

  /** Performs the request and turns a non-2xx into an ApiError. */
  private async fetchRaw(path: string, init: RequestInit = {}): Promise<Response> {
    if (!this.host?.token) {
      throw new ApiError(
        'This page is not running inside the ProfileUnity SplashScreen Logo Manager host process.',
        0,
      );
    }

    const headers = new Headers(init.headers ?? {});
    headers.set('X-PSM-Token', this.host.token);
    if (init.body !== undefined) {
      headers.set('Content-Type', 'application/json');
    }

    let response: Response;
    try {
      response = await fetch(path, { ...init, headers, cache: 'no-store' });
    } catch (cause) {
      throw new ApiError(
        `Could not reach the local service: ${cause instanceof Error ? cause.message : String(cause)}`,
        0,
      );
    }

    if (!response.ok) {
      let message = `Request failed with status ${response.status}.`;
      try {
        const body = (await response.clone().json()) as { error?: string };
        if (body?.error) {
          message = body.error;
        }
      } catch {
        const text = await response.clone().text();
        if (text.trim()) {
          message = text.trim();
        }
      }
      throw new ApiError(message, response.status);
    }
    return response;
  }

  private post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  }

  state(): Promise<AppState> {
    return this.request<AppState>('/api/state');
  }

  about(): Promise<AboutInfo> {
    return this.request<AboutInfo>('/api/about');
  }

  browse(): Promise<{ cancelled?: boolean }> {
    return this.post('/api/browse');
  }

  importClipboard(): Promise<unknown> {
    return this.post('/api/clipboard');
  }

  discardPending(): Promise<unknown> {
    return this.post('/api/discard');
  }

  apply(): Promise<{ applied: string }> {
    return this.post('/api/apply');
  }

  restore(id: string): Promise<{ restored: string }> {
    return this.post('/api/restore', { id });
  }

  deleteHistory(id: string): Promise<unknown> {
    return this.post('/api/history/delete', { id });
  }

  search(query: string): Promise<{ opened: string }> {
    return this.post('/api/search', { query });
  }

  previewSplash(): Promise<{ launched: string }> {
    return this.post('/api/preview-splash');
  }

  openDocument(which: 'license' | 'sbom' | 'notices'): Promise<unknown> {
    return this.post('/api/open-doc', { which });
  }

  /**
   * Fetches image bytes and returns an object URL.
   *
   * Images go through fetch rather than a plain <img src> so the API token
   * travels in a header instead of a query string. Callers must revoke the URL.
   */
  async imageObjectUrl(kind: 'live' | 'pending' | 'history', id?: string): Promise<string> {
    const query = new URLSearchParams({ kind });
    if (id) {
      query.set('id', id);
    }
    const response = await this.fetchRaw(`/api/image?${query.toString()}`);
    return URL.createObjectURL(await response.blob());
  }
}
