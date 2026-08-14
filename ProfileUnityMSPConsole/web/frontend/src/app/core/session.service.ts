import { Injectable, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';

export interface SessionUser {
  username: string;
  role: string;
}

/**
 * Holds the current console-operator session client-side, mirroring the
 * reference project's pattern: a root-provided shared service (not
 * component-local state) for session status that must work across every
 * screen, with a full state reset on expiry rather than leaving a stale
 * screen sitting there.
 */
@Injectable({ providedIn: 'root' })
export class SessionService {
  private readonly http = inject(HttpClient);
  private readonly router = inject(Router);

  readonly user = signal<SessionUser | null>(null);

  /** True once an initial /api/auth/me check has completed, success or not. */
  readonly checked = signal(false);

  async checkSession(): Promise<boolean> {
    try {
      const me = await firstValueFrom(this.http.get<SessionUser>('/api/auth/me'));
      this.user.set(me);
      return true;
    } catch {
      this.user.set(null);
      return false;
    } finally {
      this.checked.set(true);
    }
  }

  async login(username: string, password: string): Promise<void> {
    await firstValueFrom(this.http.get('/api/csrf'));
    const me = await firstValueFrom(
      this.http.post<SessionUser>('/api/auth/login', { username, password }),
    );
    this.user.set(me);
    this.checked.set(true);
  }

  async logout(): Promise<void> {
    try {
      await firstValueFrom(this.http.post('/api/auth/logout', {}));
    } finally {
      this.user.set(null);
      this.router.navigateByUrl('/login');
    }
  }

  /**
   * Called by the auth interceptor when a request comes back 401 outside
   * the login/me flows — the session expired server-side (idle or
   * absolute timeout). Resets state and sends the operator back to the
   * login screen with an explicit message, per the reference project's
   * idle-timeout pattern.
   */
  handleExpired(): void {
    this.user.set(null);
    this.router.navigate(['/login'], { queryParams: { expired: 1 } });
  }
}
