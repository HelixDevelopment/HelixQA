import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService, Subscription } from '../../core/api.service';
import { PageHeaderComponent, StatusBadgeComponent } from '../../shared/index';

@Component({
  selector: 'app-subscriptions',
  standalone: true,
  imports: [CommonModule, RouterLink, PageHeaderComponent, StatusBadgeComponent],
  template: `
    <div class="page">
      <app-page-header title="Subscriptions"></app-page-header>

      <div class="loading" *ngIf="loading">
        <div class="spinner"></div>
        <span>Loading subscriptions...</span>
      </div>

      <div class="error-banner" *ngIf="error">
        <span>{{ error }}</span>
        <button class="btn btn-sm" (click)="loadSubscriptions()">Retry</button>
      </div>

      <div class="table-container" *ngIf="!loading && !error">
        <table *ngIf="subscriptions.length > 0">
          <thead>
            <tr>
              <th>Customer</th>
              <th>Plan</th>
              <th>Amount</th>
              <th>Status</th>
              <th>Next Billing</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let sub of subscriptions">
              <td>
                <a routerLink="/customers" class="customer-link">
                  {{ sub.customer_id }}
                </a>
              </td>
              <td>{{ sub.plan_id }}</td>
              <td class="amount-cell">{{ sub.amount / 100 | currency:'USD':'symbol':'1.2-2' }}</td>
              <td>
                <app-status-badge [label]="sub.status" [variant]="sub.status"></app-status-badge>
              </td>
              <td>{{ sub.current_period_end | date:'mediumDate' }}</td>
              <td>
                <button class="btn btn-sm btn-danger" *ngIf="sub.status === 'active' || sub.status === 'past_due'"
                  (click)="cancelSubscription(sub)">
                  Cancel
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="subscriptions.length === 0">No subscriptions found.</div>
      </div>
    </div>
  `,
  styles: [`
    .page { padding: var(--od-spacing-xl, 24px); }
    .page-header { margin-bottom: var(--od-spacing-lg, 20px); }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary, #1a1a1a); }
    .btn {
      padding: 8px 16px; border: none; border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; cursor: pointer; font-family: var(--od-font-sans, inherit);
    }
    .btn:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-danger { background: var(--od-danger, #dc3545); color: #fff; }
    .btn-danger:hover { opacity: 0.9; }
    .btn-sm { padding: 4px 10px; font-size: 12px; }
    .loading {
      display: flex; align-items: center; gap: 12px; padding: 40px; justify-content: center;
      color: var(--od-text-muted, #999);
    }
    .spinner {
      width: 20px; height: 20px; border: 2px solid var(--od-border, #ddd);
      border-top-color: var(--od-brand-primary, #4f46e5); border-radius: 50%; animation: spin 0.6s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .error-banner {
      display: flex; justify-content: space-between; align-items: center; padding: 12px 16px;
      background: #fee2e2; color: #991b1b; border-radius: var(--od-radius, 8px); margin-bottom: 16px;
    }
    .table-container {
      background: var(--od-card-bg, #fff); border-radius: var(--od-radius, 8px);
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1)); overflow-x: auto;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; white-space: nowrap; }
    th {
      background: var(--od-bg-secondary, #f9fafb); color: var(--od-text-secondary, #666);
      font-weight: 500; border-bottom: 1px solid var(--od-border, #e5e7eb);
    }
    td { border-bottom: 1px solid var(--od-bg-tertiary, #f3f4f6); color: var(--od-text-primary, #333); }
    tr:last-child td { border-bottom: none; }
    .amount-cell { font-weight: 500; }
    .customer-link { color: var(--od-brand-primary, #4f46e5); text-decoration: none; }
    .customer-link:hover { text-decoration: underline; }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.trialing { background: #dbeafe; color: #1e40af; }
    .badge.past_due { background: #fef9c3; color: #854d0e; }
    .badge.cancelled { background: #fee2e2; color: #991b1b; }
    .badge.unpaid { background: #f3f4f6; color: #6b7280; }
    .empty-state { padding: 40px; text-align: center; color: var(--od-text-muted, #999); }
  `]
})
export class SubscriptionsComponent implements OnInit {
  subscriptions: Subscription[] = [];
  loading = true;
  error = '';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadSubscriptions();
  }

  loadSubscriptions(): void {
    this.loading = true;
    this.error = '';
    this.api.getSubscriptions().subscribe({
      next: (res) => { this.subscriptions = res.data; this.loading = false; },
      error: () => { this.error = 'Failed to load subscriptions.'; this.loading = false; },
    });
  }

  cancelSubscription(sub: Subscription): void {
    if (!confirm(`Cancel subscription ${sub.id} for ${sub.plan_id}? This action cannot be undone.`)) return;
    this.api.cancelSubscription(sub.id).subscribe({
      next: () => this.loadSubscriptions(),
      error: () => this.error = 'Failed to cancel subscription.',
    });
  }
}
