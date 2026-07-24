import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-subscriptions',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="subscriptions-page">
      <div class="page-header">
        <h1>Subscriptions</h1>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Customer</th>
              <th>Plan</th>
              <th>Amount</th>
              <th>Interval</th>
              <th>Status</th>
              <th>Current Period</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let sub of subscriptions">
              <td class="id-cell">{{ sub.id }}</td>
              <td>
                <a [routerLink]="['/merchants', sub.merchant_id, 'customers', sub.customer_id]">
                  {{ sub.customer_id }}
                </a>
              </td>
              <td>{{ sub.plan_id }}</td>
              <td class="amount-cell">\${{ sub.amount / 100 | number:'1.2-2' }}</td>
              <td>{{ sub.interval }}{{ sub.interval_count > 1 ? ' (x' + sub.interval_count + ')' : '' }}</td>
              <td>
                <span class="badge" [ngClass]="sub.status">{{ sub.status }}</span>
              </td>
              <td>{{ sub.current_period_start | date:'mediumDate' }} &mdash; {{ sub.current_period_end | date:'mediumDate' }}</td>
              <td>{{ sub.created_at | date:'mediumDate' }}</td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="subscriptions.length === 0 && !loading">No subscriptions found.</div>
      </div>
    </div>
  `,
  styles: [`
    .subscriptions-page { padding: 24px; }
    .page-header { margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .table-container {
      background: white;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      overflow-x: auto;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; white-space: nowrap; }
    th { background: #f9fafb; color: #666; font-weight: 500; border-bottom: 1px solid #e5e7eb; }
    td { border-bottom: 1px solid #f3f4f6; color: #333; }
    tr:last-child td { border-bottom: none; }
    .id-cell { font-family: monospace; font-size: 12px; max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
    .amount-cell { font-weight: 500; }
    td a { color: #4f46e5; text-decoration: none; }
    td a:hover { text-decoration: underline; }
    .badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 500;
      text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.trialing { background: #dbeafe; color: #1e40af; }
    .badge.past_due { background: #fef9c3; color: #854d0e; }
    .badge.cancelled { background: #fee2e2; color: #991b1b; }
    .badge.unpaid { background: #f3f4f6; color: #6b7280; }
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class SubscriptionsComponent implements OnInit {
  subscriptions: any[] = [];
  loading = true;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getSubscriptions().subscribe({
      next: (res) => {
        this.subscriptions = res.data;
        this.loading = false;
      },
      error: () => this.loading = false
    });
  }
}
