import { Component, inject } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';

import { SessionService } from '../core/session.service';
import { LanguageSwitcherComponent } from '../shared/language-switcher.component';

interface NavItem {
  path: string;
  labelKey: string;
}

@Component({
  selector: 'app-shell',
  standalone: true,
  imports: [RouterLink, RouterLinkActive, RouterOutlet, TranslocoModule, ButtonModule, LanguageSwitcherComponent],
  templateUrl: './shell.component.html',
})
export class ShellComponent {
  readonly session = inject(SessionService);

  readonly navItems: NavItem[] = [
    { path: '/dashboard', labelKey: 'nav.dashboard' },
    { path: '/tenants', labelKey: 'nav.tenants' },
    { path: '/history', labelKey: 'nav.history' },
    { path: '/reports', labelKey: 'nav.reports' },
    { path: '/about', labelKey: 'nav.about' },
  ];

  signOut(): void {
    void this.session.logout();
  }
}
