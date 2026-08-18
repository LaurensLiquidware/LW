import { Routes } from '@angular/router';
import { authGuard } from './core/auth.guard';

export const routes: Routes = [
  {
    path: 'login',
    loadComponent: () => import('./features/login/login.component').then((m) => m.LoginComponent),
  },
  {
    path: 'about',
    loadComponent: () => import('./features/about/about.component').then((m) => m.AboutComponent),
  },
  {
    path: '',
    loadComponent: () => import('./layout/shell.component').then((m) => m.ShellComponent),
    canActivate: [authGuard],
    children: [
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' },
      {
        path: 'dashboard',
        loadComponent: () => import('./features/dashboard/dashboard.component').then((m) => m.DashboardComponent),
      },
      {
        path: 'tenants',
        loadComponent: () => import('./features/tenants/tenants.component').then((m) => m.TenantsComponent),
      },
      {
        path: 'history',
        loadComponent: () => import('./features/history/history.component').then((m) => m.HistoryComponent),
      },
      {
        path: 'reports',
        loadComponent: () => import('./features/reports/reports.component').then((m) => m.ReportsComponent),
      },
      {
        path: 'licenses',
        loadComponent: () => import('./features/licenses/licenses.component').then((m) => m.LicensesComponent),
      },
      {
        path: 'settings',
        loadComponent: () => import('./features/settings/settings.component').then((m) => m.SettingsComponent),
      },
      {
        path: 'users',
        loadComponent: () => import('./features/users/users.component').then((m) => m.UsersComponent),
      },
      // No nested "about" route: it's shown as a dialog from the shell
      // (see shell.component.html) so it never needs a back button.
      // The top-level "about" route above stays directly reachable by URL
      // pre-login (e.g. for a compliance check before deploying), even
      // though the login screen no longer links to it.
    ],
  },
  { path: '**', redirectTo: 'login' },
];
