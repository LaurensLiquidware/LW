import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, throwError } from 'rxjs';
import { SessionService } from './session.service';

/**
 * On a 401 outside the login/me checks themselves, the session has
 * expired server-side (idle or absolute timeout) — reset client state
 * and send the operator back to /login with an explicit message, rather
 * than leaving a stale authenticated-looking screen on display.
 */
export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const session = inject(SessionService);
  const exempt = req.url.includes('/api/auth/me') || req.url.includes('/api/auth/login');

  return next(req).pipe(
    catchError((err: HttpErrorResponse) => {
      if (err.status === 401 && !exempt) {
        session.handleExpired();
      }
      return throwError(() => err);
    }),
  );
};
