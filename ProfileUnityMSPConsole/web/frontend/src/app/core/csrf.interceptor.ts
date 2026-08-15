import { HttpInterceptorFn } from '@angular/common/http';
import { readCookie } from './cookie';

/**
 * Echoes the CSRF cookie back as a header on every mutating request,
 * plus X-Requested-With — a cookie alone is not sufficient against CSRF
 * (project brief §6's carried-over pattern). GET/HEAD/OPTIONS pass
 * through untouched.
 */
export const csrfInterceptor: HttpInterceptorFn = (req, next) => {
  if (req.method === 'GET' || req.method === 'HEAD' || req.method === 'OPTIONS') {
    return next(req);
  }
  const token = readCookie('pumc_csrf');
  if (!token) {
    return next(req);
  }
  return next(
    req.clone({
      setHeaders: {
        'X-CSRF-Token': token,
        'X-Requested-With': 'XMLHttpRequest',
      },
    }),
  );
};
