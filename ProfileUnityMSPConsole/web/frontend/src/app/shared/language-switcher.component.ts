import { Component, Input, inject, signal } from '@angular/core';
import { TranslocoModule, TranslocoService } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';

/**
 * Runtime language switch, no reload — Transloco re-renders active
 * translations in place (project brief §5/§6: i18n from day one, with
 * runtime language switching).
 */
@Component({
  selector: 'app-language-switcher',
  standalone: true,
  imports: [ButtonModule, TranslocoModule],
  template: `
    <div class="flex gap-1" role="group" [attr.aria-label]="'header.language' | transloco">
      @for (lang of langs; track lang) {
        <button
          type="button"
          pButton
          size="small"
          [severity]="severity"
          [outlined]="active() !== lang"
          (click)="setLang(lang)"
        >
          {{ lang.toUpperCase() }}
        </button>
      }
    </div>
  `,
})
export class LanguageSwitcherComponent {
  private readonly transloco = inject(TranslocoService);
  readonly langs = ['en', 'nl'];
  readonly active = signal(this.transloco.getActiveLang());

  /**
   * "contrast" auto-adjusts for a colored/dark backdrop (the app header);
   * the default primary look is legible enough on its own against the
   * login screen's near-black background.
   */
  @Input() severity: 'primary' | 'contrast' = 'primary';

  setLang(lang: string): void {
    this.transloco.setActiveLang(lang);
    this.active.set(lang);
  }
}
