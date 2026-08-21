import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { TranslocoModule } from '@jsverse/transloco';
import { DialogModule } from 'primeng/dialog';

import { VersionService } from '../core/version.service';
import { LanguageSwitcherComponent } from '../shared/language-switcher.component';
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
        DialogModule,
        LanguageSwitcherComponent,
        AboutComponent,
    ],
    changeDetection: ChangeDetectionStrategy.Eager,
    templateUrl: './shell.component.html',
    styleUrl: './shell.component.css'
})
export class ShellComponent implements OnInit {
  private readonly versionService = inject(VersionService);

  readonly version = signal<string | null>(null);
  readonly aboutVisible = signal(false);

  readonly navItems: NavItem[] = [
    { path: '/dashboard', labelKey: 'nav.dashboard', icon: 'pi pi-home' },
    { path: '/new-scan', labelKey: 'nav.newScan', icon: 'pi pi-search' },
    { path: '/results', labelKey: 'nav.results', icon: 'pi pi-list' },
    { path: '/compare', labelKey: 'nav.compare', icon: 'pi pi-sort-alt' },
  ];

  async ngOnInit(): Promise<void> {
    try {
      this.version.set(await this.versionService.fetch());
    } catch {
      this.version.set(null);
    }
  }
}
