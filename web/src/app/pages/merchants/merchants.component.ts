import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink, Router } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-merchants',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="merchants-page">
      <div class="page-header">
        <h1>Merchants</h1>
        <a routerLink="/merchants/new" class="btn btn-primary">Create Merchant</a>
      </div>

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
                <span class="badge" [ngClass]="merchant.status">
                  {{ merchant.status }}
                </span>
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
    .merchants-page { padding: 24px; }
    .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .table-container {
      background: white;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; }
    th { background: #f9fafb; color: #666; font-weight: 500; border-bottom: 1px solid #e5e7eb; }
    td { border-bottom: 1px solid #f3f4f6; color: #333; }
    tr:last-child td { border-bottom: none; }
    td a { color: #4f46e5; text-decoration: none; font-weight: 500; }
    td a:hover { text-decoration: underline; }
    .clickable-row { cursor: pointer; }
    .clickable-row:hover td { background: #f9fafb; }
    .badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 500;
      text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.pending, .badge.pending_verification { background: #fef9c3; color: #854d0e; }
    .badge.suspended { background: #fee2e2; color: #991b1b; }
    .empty-state { padding: 40px; text-align: center; color: #999; }
    .spinner {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 12px;
      padding: 48px;
      color: #666;
    }
    .spinner-icon {
      width: 20px;
      height: 20px;
      border: 2px solid #e5e7eb;
      border-top-color: #4f46e5;
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
    .error-state {
      background: #fef2f2;
      border: 1px solid #fecaca;
      border-radius: 8px;
      padding: 24px;
      text-align: center;
      color: #991b1b;
    }
    .error-state button { margin-top: 12px; }
    .btn {
      padding: 8px 20px;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      text-decoration: none;
      border: none;
      transition: background-color 0.2s;
    }
    .btn-primary { background: #4f46e5; color: white; }
    .btn-primary:hover { background: #4338ca; }
    .btn-secondary { background: #f3f4f6; color: #374151; display: inline-flex; align-items: center; }
    .btn-secondary:hover { background: #e5e7eb; }
  `]
})
export class MerchantsComponent implements OnInit {
  merchants: any[] = [];
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
