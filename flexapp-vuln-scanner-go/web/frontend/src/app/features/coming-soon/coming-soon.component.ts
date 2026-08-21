import { Component, inject } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';

/**
 * Placeholder for the routes this phase only stubs out (Dashboard,
 * Tenants, History, Reports) — each lands in its own later build phase.
 * The route's `data.titleKey` picks which nav heading to show.
 */
@Component({
  selector: 'app-coming-soon',
  standalone: true,
  imports: [TranslocoModule],
  template: `
    <div class="flex flex-col items-center justify-center gap-2 p-12">
      <h1 class="lwl-card-title">{{ titleKey | transloco }}</h1>
      <p class="lwl-muted">{{ 'comingSoon.message' | transloco }}</p>
    </div>
  `,
})
export class ComingSoonComponent {
  readonly titleKey = inject(ActivatedRoute).snapshot.data['titleKey'] as string;
}
