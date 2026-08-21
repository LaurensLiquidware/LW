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
        loadComponent: () => import('./features/dashboard/dashboard.component').then((m) => m.DashboardComponent),
      },
      {
        path: 'new-scan',
        loadComponent: () => import('./features/new-scan/new-scan.component').then((m) => m.NewScanComponent),
      },
      {
        path: 'scan-progress',
        loadComponent: () =>
          import('./features/scan-progress/scan-progress.component').then((m) => m.ScanProgressComponent),
      },
      {
        path: 'results',
        loadComponent: () => import('./features/results/results.component').then((m) => m.ResultsComponent),
      },
      {
        path: 'compare',
        loadComponent: () => import('./features/compare/compare.component').then((m) => m.CompareComponent),
      },
      // No nested "about" route: it's shown as a dialog from the shell
      // (see shell.component.html) so it never needs a back button. The
      // top-level "about" route above stays directly reachable by URL.
    ],
  },
  { path: '**', redirectTo: '' },
];
