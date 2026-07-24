import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink, Router } from '@angular/router';
import { ApiService, Merchant } from '../../core/api.service';
import { PageHeaderComponent, StatusBadgeComponent } from '../../shared/index';

@Component({
  selector: 'app-merchants',
  standalone: true,
  imports: [CommonModule, RouterLink, PageHeaderComponent, StatusBadgeComponent],
  template: `
    <div class="merchants-page">
      <app-page-header title="Merchants">
        <a routerLink="/merchants/new" class="btn btn-primary">Create Merchant</a>
      </app-page-header>

      <div class="spinner" *ngIf="loading">
        <div class="spinner-icon"></div>
        <span>Loading merchants...</span>
      </div>

      <div class="error-state" *ngIf="error && !loading">
        <p>{{ error }}</p>
        <button class="btn btn-secondary" (click)="loadMerchants()">Retry</button>
      </div>

      <div class="table-container" *ngIf="!loading && !error">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Status</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let merchant of merchants" (click)="goToDetail(merchant.id)" class="clickable-row">
              <td>
                <a [routerLink]="['/merchants', merchant.id]">{{ merchant.trade_name || merchant.name }}</a>
              </td>
              <td>{{ merchant.email }}</td>
              <td>
                <app-status-badge [label]="merchant.status" [variant]="merchant.status"></app-status-badge>
              </td>
              <td>{{ merchant.created_at | date:'mediumDate' }}</td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="merchants.length === 0">No merchants found.</div>
      </div>
    </div>
  `,
  styles: [`
    .merchants-page { padding: var(--od-spacing-xl); }
    .table-container {
      background: var(--od-card-bg);
      border-radius: var(--od-radius);
      box-shadow: var(--od-card-shadow);
      overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; }
    th { background: var(--od-bg-secondary); color: var(--od-text-secondary); font-weight: 500; border-bottom: 1px solid var(--od-border); }
    td { border-bottom: 1px solid var(--od-bg-tertiary); color: var(--od-text-primary); }
    tr:last-child td { border-bottom: none; }
    td a { color: var(--od-accent); text-decoration: none; font-weight: 500; }
    td a:hover { text-decoration: underline; }
    .clickable-row { cursor: pointer; }
    .clickable-row:hover td { background: var(--od-bg-secondary); }
    .empty-state { padding: 40px; text-align: center; color: var(--od-text-muted); }
    .spinner {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 12px;
      padding: 48px;
      color: var(--od-text-secondary);
    }
    .spinner-icon {
      width: 20px;
      height: 20px;
      border: 2px solid var(--od-border);
      border-top-color: var(--od-accent);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .error-state {
      background: var(--od-bg-danger, #fef2f2);
      border: 1px solid var(--od-border-danger, #fecaca);
      border-radius: var(--od-radius);
      padding: 24px;
      text-align: center;
      color: var(--od-danger, #991b1b);
    }
    .error-state button { margin-top: 12px; }
    .btn {
      padding: 8px 20px;
      border-radius: var(--od-radius-sm);
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      text-decoration: none;
      border: none;
      transition: background-color 0.2s;
    }
    .btn-primary { background: var(--od-accent); color: white; }
    .btn-primary:hover { opacity: 0.9; }
    .btn-secondary { background: var(--od-bg-secondary); color: var(--od-text-primary); display: inline-flex; align-items: center; }
    .btn-secondary:hover { background: var(--od-bg-tertiary); }
  `]
})
export class MerchantsComponent implements OnInit {
  merchants: Merchant[] = [];
  loading = true;
  error = '';

  constructor(private api: ApiService, private router: Router) {}

  ngOnInit(): void {
    this.loadMerchants();
  }

  loadMerchants(): void {
    this.loading = true;
    this.error = '';
    this.api.getMerchants().subscribe({
      next: (res) => {
        this.merchants = res.data;
        this.loading = false;
      },
      error: () => {
        this.error = 'Failed to load merchants. Please try again.';
        this.loading = false;
      }
    });
  }

  goToDetail(id: string): void {
    this.router.navigate(['/merchants', id]);
  }
}
