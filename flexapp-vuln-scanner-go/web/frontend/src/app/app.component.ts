import { Component, ChangeDetectionStrategy, DOCUMENT, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { TranslocoService } from '@jsverse/transloco';

@Component({
    selector: 'app-root',
    imports: [RouterOutlet],
    changeDetection: ChangeDetectionStrategy.Eager,
    template: '<router-outlet />'
})
export class AppComponent {
  private readonly document = inject(DOCUMENT);
  private readonly transloco = inject(TranslocoService);

  constructor() {
    // Angular's static LOCALE_ID never reflects a runtime language
    // switch, but <html lang> is still load-bearing for screen readers,
    // browser spell-check, and CSS :lang() selectors -- keep it in sync
    // with Transloco's active language rather than leaving it frozen at
    // whatever index.html hardcoded (project brief §11's Unicode/i18n
    // compliance).
    this.document.documentElement.lang = this.transloco.getActiveLang();
    this.transloco.langChanges$.subscribe((lang) => {
      this.document.documentElement.lang = lang;
    });
  }
}
