import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';

import { SessionService } from '../core/session.service';
import { VersionService } from '../core/version.service';
import { LanguageSwitcherComponent } from '../shared/language-switcher.component';
import { AlertBellComponent } from '../shared/alert-bell.component';

interface NavItem {
  path: string;
  labelKey: string;
}

@Component({
    selector: 'app-shell',
    imports: [RouterLink, RouterLinkActive, RouterOutlet, TranslocoModule, ButtonModule, LanguageSwitcherComponent, AlertBellComponent],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './shell.component.html'
})
export class ShellComponent implements OnInit {
  readonly session = inject(SessionService);
  private readonly versionService = inject(VersionService);

  readonly version = signal<string | null>(null);

  readonly navItems: NavItem[] = [
    { path: '/dashboard', labelKey: 'nav.dashboard' },
    { path: '/tenants', labelKey: 'nav.tenants' },
    { path: '/history', labelKey: 'nav.history' },
    { path: '/reports', labelKey: 'nav.reports' },
    { path: '/about', labelKey: 'nav.about' },
  ];

  async ngOnInit(): Promise<void> {
    try {
      this.version.set(await this.versionService.fetch());
    } catch {
      this.version.set(null);
    }
  }

  signOut(): void {
    void this.session.logout();
  }
}
