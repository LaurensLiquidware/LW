import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink } from '@angular/router';
import { NavigationEnd, Router } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { PopoverModule } from 'primeng/popover';
import { OverlayBadgeModule } from 'primeng/overlaybadge';
import { ButtonModule } from 'primeng/button';
import { filter } from 'rxjs';

import { AlertsService } from '../core/alerts.service';
import { Alert } from '../core/models/alert';

/**
 * The shell-level alert indicator (project brief §7.6). In-app only, no
 * email/SMTP -- an operator sees it the moment they're in the console,
 * which was judged sufficient without the added surface area of an
 * outbound mail dependency. Refetches on every navigation so it stays
 * roughly current as an operator moves between screens, without needing
 * a poll timer or a push channel.
 */
@Component({
    selector: 'app-alert-bell',
    imports: [RouterLink, TranslocoModule, PopoverModule, OverlayBadgeModule, ButtonModule],
    changeDetection: ChangeDetectionStrategy.Eager,
    template: `
    <button
      type="button"
      class="flex items-center justify-center"
      style="width: 2.25rem; height: 2.25rem; background: transparent; border: none; cursor: pointer; color: var(--p-surface-0);"
      (click)="panel.toggle($event)"
      [attr.aria-label]="'alerts.title' | transloco"
    >
      @if (alerts().length > 0) {
        <p-overlaybadge [value]="alerts().length" severity="danger">
          <span class="material-icons">notifications</span>
        </p-overlaybadge>
      } @else {
        <span class="material-icons">notifications</span>
      }
    </button>

    <p-popover #panel>
      <div style="min-width: 20rem; max-width: 24rem;">
        <h2 class="lwl-metric-label mb-2">{{ 'alerts.title' | transloco }}</h2>
        @if (alerts().length === 0) {
          <p class="lwl-muted">{{ 'alerts.none' | transloco }}</p>
        } @else {
          <ul class="flex flex-col gap-2">
            @for (alert of alerts(); track alert.tenant.tenant.id) {
              <li>
                <a [routerLink]="['/dashboard']" class="lwl-cell" style="color: var(--p-text-color); display: block;">
                  <strong>{{ alert.tenant.tenant.displayName }}</strong>
                  <div class="lwl-micro lwl-muted">
                    @for (reason of alert.reasons; track reason) {
                      <span class="mr-2">{{ 'alerts.reason.' + reason | transloco }}</span>
                    }
                  </div>
                </a>
              </li>
            }
          </ul>
        }
      </div>
    </p-popover>
  `
})
export class AlertBellComponent implements OnInit {
  private readonly alertsService = inject(AlertsService);
  private readonly router = inject(Router);

  readonly alerts = signal<Alert[]>([]);

  ngOnInit(): void {
    void this.refresh();
    this.router.events.pipe(filter((e) => e instanceof NavigationEnd)).subscribe(() => void this.refresh());
  }

  private async refresh(): Promise<void> {
    this.alerts.set(await this.alertsService.list());
  }
}
