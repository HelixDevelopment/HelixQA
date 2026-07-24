import { Component } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive],
  template: `
    <div class="app-layout" [attr.data-theme]="theme">
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
        <div class="sidebar-footer">
          <button class="theme-toggle" (click)="toggleTheme()">
            {{ theme === 'light' ? '🌙' : '☀️' }}
            {{ theme === 'light' ? 'Dark' : 'Light' }}
          </button>
        </div>
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
      background: var(--od-sidebar-bg);
      color: var(--od-sidebar-text);
      display: flex;
      flex-direction: column;
    }

    .sidebar-brand {
      padding: var(--od-spacing-lg);
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
      padding: var(--od-spacing-md) 0;
      flex: 1;
    }

    .sidebar-nav a {
      padding: 0.75rem var(--od-spacing-lg);
      color: var(--od-sidebar-text);
      opacity: 0.7;
      text-decoration: none;
      transition: background 0.2s, color 0.2s;
    }

    .sidebar-nav a:hover {
      background: var(--od-sidebar-hover);
      color: #ffffff;
      opacity: 1;
    }

    .sidebar-nav a.active {
      background: var(--od-sidebar-active);
      color: #ffffff;
      opacity: 1;
      border-left: 3px solid var(--od-accent);
    }

    .sidebar-footer {
      padding: var(--od-spacing-md) var(--od-spacing-lg);
      border-top: 1px solid rgba(255, 255, 255, 0.1);
    }

    .theme-toggle {
      width: 100%;
      padding: 0.5rem;
      border: 1px solid rgba(255, 255, 255, 0.2);
      border-radius: var(--od-radius-sm);
      background: transparent;
      color: var(--od-sidebar-text);
      cursor: pointer;
      font-family: inherit;
      font-size: 0.875rem;
      transition: background 0.2s;
    }

    .theme-toggle:hover {
      background: var(--od-sidebar-hover);
    }

    .content {
      flex: 1;
      background: var(--od-bg-secondary);
      padding: var(--od-spacing-xl);
    }
  `,
})
export class AppComponent {
  theme: 'light' | 'dark' = 'light';

  toggleTheme(): void {
    this.theme = this.theme === 'light' ? 'dark' : 'light';
  }
}
