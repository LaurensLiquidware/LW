import { ApplicationConfig, provideZoneChangeDetection, isDevMode } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { providePrimeNG } from 'primeng/config';
import { provideTransloco } from '@jsverse/transloco';

import { routes } from './app.routes';
import { LiquidwarePreset } from './theme/liquidware-preset';
import { TranslocoHttpLoader } from './transloco-loader';
import { csrfInterceptor } from './core/csrf.interceptor';
import { authInterceptor } from './core/auth.interceptor';

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([csrfInterceptor, authInterceptor])),
    provideAnimationsAsync(),
    providePrimeNG({
      theme: {
        preset: LiquidwarePreset,
        options: {
          darkModeSelector: '.dark',
        },
      },
    }),
    provideTransloco({
      config: {
        availableLangs: ['en', 'nl'],
        defaultLang: 'en',
        fallbackLang: 'en',
        reRenderOnLangChange: true,
        prodMode: !isDevMode(),
      },
      loader: TranslocoHttpLoader,
    }),
  ],
};
