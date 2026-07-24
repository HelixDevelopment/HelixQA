import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-merchant-detail',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="merchant-detail" *ngIf="merchant">
      <div class="page-header">
        <a routerLink="/merchants" class="back-link">&larr; Back to Merchants</a>
        <h1>{{ merchant.trade_name || merchant.name }}</h1>
      </div>
      <div class="detail-grid">
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
    .merchant-detail { padding: 24px; }
    .page-header { margin-bottom: 24px; }
    .back-link { color: #4f46e5; text-decoration: none; font-size: 14px; display: inline-block; margin-bottom: 8px; }
    .back-link:hover { text-decoration: underline; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .detail-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
      gap: 16px;
    }
    .detail-card {
      background: white;
      border-radius: 8px;
      padding: 20px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .detail-card h3 { margin: 0 0 16px; font-size: 16px; color: #333; }
    dl { margin: 0; }
    dt { font-size: 12px; color: #666; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 4px; }
    dd { margin: 0 0 16px; font-size: 14px; color: #1a1a1a; }
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
  `]
})
export class MerchantDetailComponent implements OnInit {
  merchant: any = null;

  constructor(private route: ActivatedRoute, private api: ApiService) {}

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('merchantId')!;
    this.api.getMerchant(id).subscribe({
      next: (data) => this.merchant = data,
      error: () => {}
    });
  }
}
