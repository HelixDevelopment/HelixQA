import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="dashboard">
      <h1>Dashboard</h1>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-label">Revenue</div>
          <div class="stat-value">\${{ summary.total_revenue | number:'1.2-2' }}</div>
          <div class="stat-period">{{ summary.period }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Transactions</div>
          <div class="stat-value">{{ summary.total_transactions | number }}</div>
          <div class="stat-period">{{ summary.period }}</div>
        </div>
        <div class="stat-card success">
          <div class="stat-label">Successful</div>
          <div class="stat-value">{{ summary.successful_transactions | number }}</div>
          <div class="stat-period">{{ summary.period }}</div>
        </div>
        <div class="stat-card danger">
          <div class="stat-label">Failed</div>
          <div class="stat-value">{{ summary.failed_transactions | number }}</div>
          <div class="stat-period">{{ summary.period }}</div>
        </div>
      </div>
      <div class="quick-links">
        <h2>Quick Links</h2>
        <div class="links-grid">
          <a routerLink="/merchants" class="link-card">
            <span class="link-title">Merchants</span>
            <span class="link-desc">Manage merchant accounts</span>
          </a>
          <a routerLink="/transactions" class="link-card">
            <span class="link-title">Transactions</span>
            <span class="link-desc">View all transactions</span>
          </a>
          <a routerLink="/customers" class="link-card">
            <span class="link-title">Customers</span>
            <span class="link-desc">Customer management</span>
          </a>
          <a routerLink="/subscriptions" class="link-card">
            <span class="link-title">Subscriptions</span>
            <span class="link-desc">Subscription management</span>
          </a>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .dashboard { padding: 24px; }
    h1 { margin: 0 0 24px; font-size: 24px; color: #1a1a1a; }
    .stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 16px;
      margin-bottom: 32px;
    }
    .stat-card {
      background: white;
      border-radius: 8px;
      padding: 20px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .stat-card.success { border-left: 4px solid #16a34a; }
    .stat-card.danger { border-left: 4px solid #dc2626; }
    .stat-label { font-size: 13px; color: #666; text-transform: uppercase; letter-spacing: 0.5px; }
    .stat-value { font-size: 28px; font-weight: 600; color: #1a1a1a; margin: 8px 0 4px; }
    .stat-period { font-size: 12px; color: #999; }
    .quick-links h2 { font-size: 18px; margin: 0 0 16px; color: #333; }
    .links-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
      gap: 12px;
    }
    .link-card {
      display: block;
      background: white;
      border-radius: 8px;
      padding: 16px;
      text-decoration: none;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      transition: box-shadow 0.2s;
    }
    .link-card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.15); }
    .link-title { display: block; font-size: 15px; font-weight: 500; color: #4f46e5; }
    .link-desc { display: block; font-size: 13px; color: #666; margin-top: 4px; }
  `]
})
export class DashboardComponent implements OnInit {
  summary = {
    total_revenue: 0,
    total_transactions: 0,
    successful_transactions: 0,
    failed_transactions: 0,
    average_transaction_size: 0,
    refund_amount: 0,
    period: ''
  };

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getAnalyticsSummary().subscribe({
      next: (data) => this.summary = { ...this.summary, ...data },
      error: () => {}
    });
  }
}
