import { Component, Input, ChangeDetectionStrategy } from '@angular/core';
import { TranslocoModule } from '@jsverse/transloco';

/**
 * Renders one of the project brief §7.3 states using the Good/Fair/Poor
 * design language (§10) for usage/expiry — but "data" status (stale,
 * failing, never collected) and a monthly report's "coverage" (complete,
 * partial, none — §7.5) are deliberately never GFP-colored: they mean we
 * don't know (or don't fully know), not that something is bad, and must
 * stay visually distinct from "poor" so a console going dark never looks
 * the same as one merely over its license limit.
 */
@Component({
    selector: 'app-status-badge',
    imports: [TranslocoModule],
    changeDetection: ChangeDetectionStrategy.Eager,
    template: `
    <span
      class="inline-flex items-center gap-1 rounded-sm px-2 py-0.5"
      style="font-size: var(--text-xs); font-weight: var(--weight-semibold);"
      [style.background]="background"
      [style.color]="color"
    >
      <span class="material-icons" style="font-size: 1em;">{{ icon }}</span>
      {{ 'status.' + kind + '.' + value | transloco }}
    </span>
  `
})
export class StatusBadgeComponent {
  @Input({ required: true }) kind!: 'usage' | 'expiry' | 'data' | 'coverage';
  @Input({ required: true }) value!: string;

  private static readonly NEUTRAL_BG = 'var(--p-surface-200)';
  private static readonly NEUTRAL_FG = 'var(--p-surface-700)';

  get color(): string {
    switch (this.value) {
      case 'good':
      case 'ok':
        return 'var(--good-color)';
      case 'fair':
      case 'expiring_soon':
        return 'var(--fair-color)';
      case 'poor':
      case 'expired':
        return 'var(--poor-color)';
      case 'unlimited':
        // Deliberately outside the Good/Fair/Poor palette, like the
        // "data"/"coverage" kinds -- an unlimited license isn't good or
        // bad, it's a different kind of license with no ceiling to judge
        // against.
        return StatusBadgeComponent.NEUTRAL_FG;
      default:
        return StatusBadgeComponent.NEUTRAL_FG;
    }
  }

  get background(): string {
    switch (this.value) {
      case 'good':
      case 'ok':
        return 'color-mix(in srgb, var(--good-color), white 85%)';
      case 'fair':
      case 'expiring_soon':
        return 'color-mix(in srgb, var(--fair-color), white 85%)';
      case 'poor':
      case 'expired':
        return 'color-mix(in srgb, var(--poor-color), white 85%)';
      case 'unlimited':
        return StatusBadgeComponent.NEUTRAL_BG;
      default:
        return StatusBadgeComponent.NEUTRAL_BG;
    }
  }

  get icon(): string {
    switch (this.value) {
      case 'good':
      case 'ok':
        return 'check_circle';
      case 'fair':
      case 'expiring_soon':
        return 'warning';
      case 'poor':
      case 'expired':
        return 'error';
      case 'stale':
        return 'schedule';
      case 'failing':
        return 'sync_problem';
      case 'never_collected':
        return 'help';
      case 'complete':
        return 'check_circle';
      case 'partial':
        return 'warning';
      case 'none':
        return 'sync_problem';
      case 'unlimited':
        return 'all_inclusive';
      default:
        return 'help';
    }
  }
}
