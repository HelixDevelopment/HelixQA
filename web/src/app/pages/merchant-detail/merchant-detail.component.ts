import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink, Router } from '@angular/router';
import { ApiService, Merchant } from '../../core/api.service';

@Component({
  selector: 'app-merchant-detail',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="merchant-detail">
      <div class="page-header">
        <a routerLink="/merchants" class="back-link">&larr; Back to Merchants</a>
        <div class="header-actions" *ngIf="merchant">
          <h1>{{ merchant.trade_name || merchant.name }}</h1>
          <div class="action-buttons">
            <a [routerLink]="['/merchants', merchant.id, 'edit']" class="btn btn-secondary">Edit</a>
            <button class="btn btn-danger" (click)="deleteMerchant()">Delete</button>
          </div>
        </div>
      </div>

      <div class="spinner" *ngIf="loading">
        <div class="spinner-icon"></div>
        <span>Loading merchant...</span>
      </div>

      <div class="error-state" *ngIf="error && !loading">
        <p>{{ error }}</p>
        <a routerLink="/merchants" class="btn btn-secondary">Back to Merchants</a>
      </div>

      <div class="detail-grid" *ngIf="merchant && !loading">
        <div class="detail-card">
          <h3>General Information</h3>
          <dl>
            <dt>Legal Name</dt>
            <dd>{{ merchant.legal_name }}</dd>
            <dt>Trade Name</dt>
            <dd>{{ merchant.trade_name || '-' }}</dd>
            <dt>Email</dt>
            <dd>{{ merchant.email }}</dd>
            <dt>Phone</dt>
            <dd>{{ merchant.phone || '-' }}</dd>
            <dt>Country</dt>
            <dd>{{ merchant.country }}</dd>
            <dt>Currency</dt>
            <dd>{{ merchant.currency }}</dd>
            <dt>Timezone</dt>
            <dd>{{ merchant.timezone || '-' }}</dd>
          </dl>
        </div>
        <div class="detail-card">
          <h3>Status</h3>
          <dl>
            <dt>Status</dt>
            <dd>
              <span class="badge" [ngClass]="merchant.status">{{ merchant.status }}</span>
            </dd>
            <dt>KYC Status</dt>
            <dd>
              <span class="badge" [ngClass]="merchant.kyc_status">{{ merchant.kyc_status }}</span>
            </dd>
            <dt>Created</dt>
            <dd>{{ merchant.created_at | date:'medium' }}</dd>
            <dt>Updated</dt>
            <dd>{{ merchant.updated_at | date:'medium' }}</dd>
          </dl>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .merchant-detail { padding: var(--od-spacing-xl); }
    .page-header { margin-bottom: var(--od-spacing-lg); }
    .back-link { color: var(--od-accent); text-decoration: none; font-size: 14px; display: inline-block; margin-bottom: 8px; }
    .back-link:hover { text-decoration: underline; }
    .header-actions { display: flex; justify-content: space-between; align-items: flex-start; }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary); }
    .action-buttons { display: flex; gap: 8px; }
    .detail-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 16px;
    }
    .detail-card {
      background: var(--od-card-bg);
      border-radius: var(--od-radius);
      padding: 20px;
      box-shadow: var(--od-card-shadow);
    }
    .detail-card h3 { margin: 0 0 16px; font-size: 16px; color: var(--od-text-primary); }
    dl { margin: 0; }
    dt { font-size: 12px; color: var(--od-text-secondary); text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 4px; }
    dd { margin: 0 0 16px; font-size: 14px; color: var(--od-text-primary); }
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
    .badge.verified { background: #dcfce7; color: #166534; }
    .badge.rejected { background: #fee2e2; color: #991b1b; }
    .badge.in_progress { background: #dbeafe; color: #1e40af; }
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
    .error-state a { margin-top: 12px; display: inline-block; }
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
    .btn-secondary { background: var(--od-bg-secondary); color: var(--od-text-primary); }
    .btn-secondary:hover { background: var(--od-bg-tertiary); }
    .btn-danger { background: var(--od-danger, #ef4444); color: white; }
    .btn-danger:hover { opacity: 0.9; }
  `]
})
export class MerchantDetailComponent implements OnInit {
  merchant: Merchant | null = null;
  loading = true;
  error = '';

  constructor(
    private route: ActivatedRoute,
    private api: ApiService,
    private router: Router
  ) {}

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id')!;
    if (!id) {
      this.error = 'Merchant ID not found.';
      this.loading = false;
      return;
    }
    this.api.getMerchant(id).subscribe({
      next: (data) => {
        this.merchant = data;
        this.loading = false;
      },
      error: () => {
        this.error = 'Failed to load merchant.';
        this.loading = false;
      }
    });
  }

  deleteMerchant(): void {
    if (!this.merchant) return;
    const confirmed = window.confirm(
      `Are you sure you want to delete "${this.merchant.trade_name || this.merchant.name}"? This action cannot be undone.`
    );
    if (!confirmed) return;
    this.api.deleteMerchant(this.merchant.id).subscribe({
      next: () => this.router.navigate(['/merchants']),
      error: () => {
        this.error = 'Failed to delete merchant. Please try again.';
      }
    });
  }
}
