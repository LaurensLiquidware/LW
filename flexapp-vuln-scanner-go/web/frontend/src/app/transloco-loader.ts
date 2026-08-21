import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Translation, TranslocoLoader } from '@jsverse/transloco';

/**
 * Loads translation JSON from /assets/i18n/{lang}.json — bundled with the
 * app, not fetched from any external translation service.
 */
@Injectable({ providedIn: 'root' })
export class TranslocoHttpLoader implements TranslocoLoader {
  constructor(private http: HttpClient) {}

  getTranslation(lang: string) {
    return this.http.get<Translation>(`assets/i18n/${lang}.json`);
  }
}
