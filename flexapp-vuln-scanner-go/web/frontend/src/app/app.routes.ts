import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: 'about',
    loadComponent: () => import('./features/about/about.component').then((m) => m.AboutComponent),
  },
  {
    path: '',
    loadComponent: () => import('./layout/shell.component').then((m) => m.ShellComponent),
    children: [
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' },
      {
        path: 'dashboard',
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.dashboard' },
      },
      {
        path: 'new-scan',
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.newScan' },
      },
      {
        path: 'results',
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.results' },
      },
      {
        path: 'compare',
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.compare' },
      },
      // No nested "about" route: it's shown as a dialog from the shell
      // (see shell.component.html) so it never needs a back button. The
      // top-level "about" route above stays directly reachable by URL.
    ],
  },
  { path: '**', redirectTo: '' },
];
