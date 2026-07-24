import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ApiService, AnalyticsSummary, Transaction, ProviderHealth } from '../../core/api.service';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="dashboard">
      <h1>Dashboard</h1>

      <div *ngIf="loading" class="od-loading">Loading dashboard data...</div>

      <div *ngIf="error" class="od-error">
        <p>{{ error }}</p>
        <button class="od-btn od-btn-secondary" (click)="ngOnInit()">Retry</button>
      </div>

      <ng-container *ngIf="!loading && !error">
        <div class="stats-grid">
          <div class="od-stat-card">
            <div class="stat-label">Total Revenue</div>
            <div class="stat-value currency">\${{ summary.total_revenue | number:'1.2-2' }}</div>
            <div class="stat-period">{{ summary.period }}</div>
          </div>
          <div class="od-stat-card">
            <div class="stat-label">Transactions</div>
            <div class="stat-value">{{ summary.total_transactions | number }}</div>
            <div class="stat-period">{{ summary.period }}</div>
          </div>
          <div class="od-stat-card od-stat-success">
            <div class="stat-label">Success Rate</div>
            <div class="stat-value">{{ summary.success_rate }}%</div>
            <div class="stat-period">{{ summary.period }}</div>
          </div>
          <div class="od-stat-card">
            <div class="stat-label">Active Merchants</div>
            <div class="stat-value">{{ summary.active_merchants | number }}</div>
            <div class="stat-period">{{ summary.period }}</div>
          </div>
        </div>

        <div class="od-section">
          <h2>Recent Transactions</h2>
          <div class="od-table-wrapper" *ngIf="recentTransactions.length > 0; else noTransactions">
            <table class="od-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Amount</th>
                  <th>Status</th>
                  <th>Provider</th>
                  <th>Date</th>
                </tr>
              </thead>
              <tbody>
                <tr *ngFor="let t of recentTransactions">
                  <td class="cell-id">{{ t.id | slice:0:8 }}...</td>
                  <td>\${{ t.amount | number:'1.2-2' }}</td>
                  <td><span class="od-status-badge" [class.od-status-success]="t.status === 'completed'" [class.od-status-pending]="t.status === 'pending'" [class.od-status-failed]="t.status === 'failed'">{{ t.status }}</span></td>
                  <td>{{ t.provider }}</td>
                  <td>{{ t.created_at | date:'MMM d, y h:mm a' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <ng-template #noTransactions>
            <p class="od-empty">No recent transactions.</p>
          </ng-template>
        </div>

        <div class="od-section">
          <h2>Provider Health</h2>
          <div class="provider-grid" *ngIf="providerHealth.length > 0; else noProviders">
            <div class="od-provider-card" *ngFor="let p of providerHealth">
              <div class="provider-info">
                <span class="provider-name">{{ p.provider }}</span>
                <span class="od-provider-status" [class.healthy]="p.status === 'healthy'" [class.degraded]="p.status === 'degraded'" [class.down]="p.status === 'down'">
                  {{ p.status }}
                </span>
              </div>
            </div>
          </div>
          <ng-template #noProviders>
            <p class="od-empty">No providers configured.</p>
          </ng-template>
        </div>

        <div class="od-section">
          <h2>Quick Links</h2>
          <div class="links-grid">
            <a routerLink="/merchants" class="od-link-card">
              <span class="link-title">Merchants</span>
              <span class="link-desc">Manage merchant accounts</span>
            </a>
            <a routerLink="/transactions" class="od-link-card">
              <span class="link-title">Transactions</span>
              <span class="link-desc">View all transactions</span>
            </a>
            <a routerLink="/customers" class="od-link-card">
              <span class="link-title">Customers</span>
              <span class="link-desc">Customer management</span>
            </a>
            <a routerLink="/subscriptions" class="od-link-card">
              <span class="link-title">Subscriptions</span>
              <span class="link-desc">Subscription management</span>
            </a>
          </div>
        </div>
      </ng-container>
    </div>
  `,
  styles: [`
    .dashboard {
      padding: var(--od-spacing-lg, 24px);
    }
    h1 {
      margin: 0 0 var(--od-spacing-lg, 24px);
      font-size: 24px;
      color: var(--od-text-primary, #1a1a1a);
    }
    h2 {
      font-size: 18px;
      margin: 0 0 var(--od-spacing-md, 16px);
      color: var(--od-text-primary, #333);
    }
    .od-loading, .od-error {
      padding: var(--od-spacing-xl, 40px);
      text-align: center;
      color: var(--od-text-secondary, #666);
    }
    .od-error {
      color: var(--od-danger, #dc3545);
    }
    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--od-spacing-md, 16px);
      margin-bottom: var(--od-spacing-xl, 32px);
    }
    .od-stat-card {
      background: var(--od-card-bg, white);
      border-radius: var(--od-radius, 8px);
      padding: var(--od-spacing-lg, 20px);
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
      border-left: 4px solid var(--od-accent, #4a90d9);
    }
    .od-stat-success {
      border-left-color: var(--od-success, #28a745);
    }
    .stat-label {
      font-size: 13px;
      color: var(--od-text-secondary, #666);
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }
    .stat-value {
      font-size: 28px;
      font-weight: 600;
      color: var(--od-text-primary, #1a1a1a);
      margin: 8px 0 4px;
    }
    .stat-value.currency {
      color: var(--od-success, #28a745);
    }
    .stat-period {
      font-size: 12px;
      color: var(--od-text-muted, #999);
    }
    .od-section {
      margin-bottom: var(--od-spacing-xl, 32px);
    }
    .od-table-wrapper {
      overflow-x: auto;
    }
    .od-table {
      width: 100%;
      border-collapse: collapse;
      background: var(--od-card-bg, white);
      border-radius: var(--od-radius, 8px);
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
    }
    .od-table th {
      text-align: left;
      padding: 12px 16px;
      font-size: 12px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--od-text-secondary, #666);
      border-bottom: 2px solid var(--od-border, #dee2e6);
    }
    .od-table td {
      padding: 12px 16px;
      font-size: 14px;
      color: var(--od-text-primary, #333);
      border-bottom: 1px solid var(--od-border, #dee2e6);
    }
    .od-table tr:last-child td {
      border-bottom: none;
    }
    .cell-id {
      font-family: var(--od-font-mono, monospace);
      font-size: 13px;
      color: var(--od-text-secondary, #666);
    }
    .od-status-badge {
      display: inline-block;
      padding: 2px 8px;
      border-radius: var(--od-radius-sm, 4px);
      font-size: 12px;
      font-weight: 500;
      text-transform: capitalize;
      background: var(--od-bg-tertiary, #e9ecef);
      color: var(--od-text-secondary, #666);
    }
    .od-status-success {
      background: rgba(40, 167, 69, 0.1);
      color: var(--od-success, #28a745);
    }
    .od-status-pending {
      background: rgba(255, 193, 7, 0.1);
      color: var(--od-warning, #856404);
    }
    .od-status-failed {
      background: rgba(220, 53, 69, 0.1);
      color: var(--od-danger, #dc3545);
    }
    .provider-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: var(--od-spacing-md, 16px);
    }
    .od-provider-card {
      background: var(--od-card-bg, white);
      border-radius: var(--od-radius, 8px);
      padding: var(--od-spacing-md, 16px);
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
    }
    .provider-info {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }
    .provider-name {
      font-weight: 500;
      color: var(--od-text-primary, #333);
      text-transform: capitalize;
    }
    .od-provider-status {
      font-size: 12px;
      font-weight: 500;
      padding: 2px 8px;
      border-radius: var(--od-radius-sm, 4px);
      text-transform: capitalize;
      background: var(--od-bg-tertiary, #e9ecef);
      color: var(--od-text-secondary, #666);
    }
    .od-provider-status.healthy {
      background: rgba(40, 167, 69, 0.1);
      color: var(--od-success, #28a745);
    }
    .od-provider-status.degraded {
      background: rgba(255, 193, 7, 0.1);
      color: var(--od-warning, #856404);
    }
    .od-provider-status.down {
      background: rgba(220, 53, 69, 0.1);
      color: var(--od-danger, #dc3545);
    }
    .od-empty {
      color: var(--od-text-muted, #999);
      font-size: 14px;
      text-align: center;
      padding: var(--od-spacing-lg, 20px);
    }
    .od-btn {
      padding: 8px 16px;
      border: none;
      border-radius: var(--od-radius-sm, 6px);
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      margin-top: var(--od-spacing-sm, 8px);
    }
    .od-btn-secondary {
      background: var(--od-bg-tertiary, #e9ecef);
      color: var(--od-text-primary, #333);
    }
    .od-btn-secondary:hover {
      filter: brightness(0.95);
    }
    .links-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: var(--od-spacing-sm, 12px);
    }
    .od-link-card {
      display: block;
      background: var(--od-card-bg, white);
      border-radius: var(--od-radius, 8px);
      padding: var(--od-spacing-md, 16px);
      text-decoration: none;
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
      transition: box-shadow 0.2s;
    }
    .od-link-card:hover {
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    }
    .link-title {
      display: block;
      font-size: 15px;
      font-weight: 500;
      color: var(--od-accent, #4a90d9);
    }
    .link-desc {
      display: block;
      font-size: 13px;
      color: var(--od-text-secondary, #666);
      margin-top: 4px;
    }
  `]
})
export class DashboardComponent implements OnInit {
  loading = true;
  error = '';

  summary: AnalyticsSummary = {
    total_revenue: 0,
    total_transactions: 0,
    successful_transactions: 0,
    failed_transactions: 0,
    success_rate: 0,
    active_merchants: 0,
    period: ''
  };

  recentTransactions: Transaction[] = [];
  providerHealth: ProviderHealth[] = [];

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loading = true;
    this.error = '';
    forkJoin({
      summary: this.api.get<AnalyticsSummary>('/analytics/summary'),
      transactions: this.api.get<{ data: Transaction[] }>('/transactions', { per_page: 5 }),
      health: this.api.get<ProviderHealth[]>('/providers/health')
    }).subscribe({
      next: (res) => {
        this.summary = { ...this.summary, ...res.summary };
        this.recentTransactions = res.transactions.data || [];
        this.providerHealth = res.health || [];
        this.loading = false;
      },
      error: () => {
        this.error = 'Failed to load dashboard data.';
        this.loading = false;
      }
    });
  }
}
