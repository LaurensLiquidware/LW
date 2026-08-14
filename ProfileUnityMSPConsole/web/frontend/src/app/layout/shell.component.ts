import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { ButtonModule } from 'primeng/button';
import { DialogModule } from 'primeng/dialog';

import { SessionService } from '../core/session.service';
import { VersionService } from '../core/version.service';
import { LanguageSwitcherComponent } from '../shared/language-switcher.component';
import { AlertBellComponent } from '../shared/alert-bell.component';
import { ChangePasswordFormComponent } from '../shared/change-password-form.component';
import { AboutComponent } from '../features/about/about.component';

interface NavItem {
  path: string;
  labelKey: string;
  icon: string;
}

@Component({
    selector: 'app-shell',
    imports: [
        RouterLink,
        RouterLinkActive,
        RouterOutlet,
        TranslocoModule,
        ButtonModule,
        DialogModule,
        LanguageSwitcherComponent,
        AlertBellComponent,
        ChangePasswordFormComponent,
        AboutComponent,
    ],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './shell.component.html',
    styleUrl: './shell.component.css'
})
export class ShellComponent implements OnInit {
  readonly session = inject(SessionService);
  private readonly versionService = inject(VersionService);

  readonly version = signal<string | null>(null);
  readonly aboutVisible = signal(false);
  readonly changePasswordVisible = signal(false);

  readonly navItems: NavItem[] = [
    { path: '/dashboard', labelKey: 'nav.dashboard', icon: 'pi pi-home' },
    { path: '/tenants', labelKey: 'nav.tenants', icon: 'pi pi-building' },
    { path: '/history', labelKey: 'nav.history', icon: 'pi pi-chart-line' },
    { path: '/reports', labelKey: 'nav.reports', icon: 'pi pi-file' },
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
