import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { SessionService } from './session.service';

/**
 * Reactive query-param reading aside, this guard itself is intentionally
 * simple: check the in-memory session first, and only hit the network if
 * this is a fresh load (checked() is false). A route reused by the
 * router (same guard re-evaluated) never redundantly re-checks.
 */
export const authGuard: CanActivateFn = async () => {
  const session = inject(SessionService);
  const router = inject(Router);

  if (session.user()) {
    return true;
  }
  if (!session.checked()) {
    const ok = await session.checkSession();
    if (ok) {
      return true;
    }
  }
  return router.createUrlTree(['/login']);
};
