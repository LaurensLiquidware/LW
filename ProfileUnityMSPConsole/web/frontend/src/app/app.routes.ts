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
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.history' },
      },
      {
        path: 'reports',
        loadComponent: () => import('./features/coming-soon/coming-soon.component').then((m) => m.ComingSoonComponent),
        data: { titleKey: 'nav.reports' },
      },
      {
        path: 'about',
        loadComponent: () => import('./features/about/about.component').then((m) => m.AboutComponent),
      },
    ],
  },
  { path: '**', redirectTo: 'login' },
];
