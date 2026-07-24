import { Routes } from '@angular/router';
import { authGuard } from './core/auth.guard';

export const routes: Routes = [
  {
    path: 'login',
    loadComponent: () =>
      import('./pages/login/login.component').then(m => m.LoginComponent),
  },
  {
    path: '',
    canActivate: [authGuard],
    children: [
      { path: '', redirectTo: 'dashboard', pathMatch: 'full' },
      {
        path: 'dashboard',
        loadComponent: () =>
          import('./pages/dashboard/dashboard.component').then(m => m.DashboardComponent),
      },
      {
        path: 'merchants',
        loadComponent: () =>
          import('./pages/merchants/merchants.component').then(m => m.MerchantsComponent),
      },
      {
        path: 'merchants/new',
        loadComponent: () =>
          import('./pages/merchant-create/merchant-create.component').then(m => m.MerchantCreateComponent),
      },
      {
        path: 'merchants/:id',
        loadComponent: () =>
          import('./pages/merchant-detail/merchant-detail.component').then(m => m.MerchantDetailComponent),
      },
      {
        path: 'merchants/:id/edit',
        loadComponent: () =>
          import('./pages/merchant-edit/merchant-edit.component').then(m => m.MerchantEditComponent),
      },
      {
        path: 'transactions',
        loadComponent: () =>
          import('./pages/transactions/transactions.component').then(m => m.TransactionsComponent),
      },
      {
        path: 'customers',
        loadComponent: () =>
          import('./pages/customers/customers.component').then(m => m.CustomersComponent),
      },
      {
        path: 'subscriptions',
        loadComponent: () =>
          import('./pages/subscriptions/subscriptions.component').then(m => m.SubscriptionsComponent),
      },
      {
        path: 'providers',
        loadComponent: () =>
          import('./pages/providers/providers.component').then(m => m.ProvidersComponent),
      },
      {
        path: 'webhooks',
        loadComponent: () =>
          import('./pages/webhooks/webhooks.component').then(m => m.WebhooksComponent),
      },
      {
        path: 'settings',
        loadComponent: () =>
          import('./pages/settings/settings.component').then(m => m.SettingsComponent),
      },
    ],
  },
  { path: '**', redirectTo: '' },
];
