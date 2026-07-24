import { Component } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive],
  template: `
    <div class="app-layout">
      <aside class="sidebar">
        <div class="sidebar-brand">
          <h1>Helix Seller</h1>
        </div>
        <nav class="sidebar-nav">
          <a routerLink="/dashboard" routerLinkActive="active">Dashboard</a>
          <a routerLink="/merchants" routerLinkActive="active">Merchants</a>
          <a routerLink="/transactions" routerLinkActive="active">Transactions</a>
          <a routerLink="/customers" routerLinkActive="active">Customers</a>
          <a routerLink="/subscriptions" routerLinkActive="active">Subscriptions</a>
          <a routerLink="/settings" routerLinkActive="active">Settings</a>
        </nav>
      </aside>
      <main class="content">
        <router-outlet />
      </main>
    </div>
  `,
  styles: `
    .app-layout {
      display: flex;
      min-height: 100vh;
    }

    .sidebar {
      width: 240px;
      background: #1a1a2e;
      color: #e0e0e0;
      display: flex;
      flex-direction: column;
    }

    .sidebar-brand {
      padding: 1.5rem;
      border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }

    .sidebar-brand h1 {
      margin: 0;
      font-size: 1.25rem;
      font-weight: 600;
    }

    .sidebar-nav {
      display: flex;
      flex-direction: column;
      padding: 1rem 0;
    }

    .sidebar-nav a {
      padding: 0.75rem 1.5rem;
      color: #a0a0b0;
      text-decoration: none;
      transition: background 0.2s, color 0.2s;
    }

    .sidebar-nav a:hover {
      background: rgba(255, 255, 255, 0.05);
      color: #ffffff;
    }

    .sidebar-nav a.active {
      background: rgba(255, 255, 255, 0.1);
      color: #ffffff;
      border-left: 3px solid #4a90d9;
    }

    .content {
      flex: 1;
      background: #f5f5f5;
      padding: 2rem;
    }
  `,
})
export class AppComponent {}
