import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink, Router, ActivatedRoute } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-merchant-edit',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  template: `
    <div class="merchant-edit">
      <div class="loading" *ngIf="!loaded">
        <div class="spinner-icon"></div>
        <span>Loading merchant...</span>
      </div>

      <ng-container *ngIf="loaded">
        <div class="page-header">
          <a routerLink="/merchants" class="back-link">&larr; Back to Merchants</a>
          <h1>Edit Merchant</h1>
        </div>

        <div class="error-banner" *ngIf="error">
          {{ error }}
        </div>

        <div class="form-card">
          <form (ngSubmit)="onSubmit()" #editForm="ngForm">
            <div class="form-grid">
              <div class="form-group">
                <label for="legal_name">Legal Name *</label>
                <input id="legal_name" type="text" [(ngModel)]="formData.legal_name" name="legal_name" #legalName="ngModel" required>
                <span class="field-error" *ngIf="legalName.invalid && legalName.touched">Legal name is required</span>
              </div>
              <div class="form-group">
                <label for="email">Email *</label>
                <input id="email" type="email" [(ngModel)]="formData.email" name="email" #email="ngModel" required email>
                <span class="field-error" *ngIf="email.invalid && email.touched">Valid email is required</span>
              </div>
              <div class="form-group">
                <label for="trade_name">Trade Name</label>
                <input id="trade_name" type="text" [(ngModel)]="formData.trade_name" name="trade_name">
              </div>
              <div class="form-group">
                <label for="phone">Phone</label>
                <input id="phone" type="tel" [(ngModel)]="formData.phone" name="phone">
              </div>
              <div class="form-group">
                <label for="country">Country</label>
                <select id="country" [(ngModel)]="formData.country" name="country">
                  <option value="" disabled>Select country</option>
                  <option value="US">United States</option>
                  <option value="GB">United Kingdom</option>
                  <option value="DE">Germany</option>
                  <option value="FR">France</option>
                </select>
              </div>
              <div class="form-group">
                <label for="currency">Currency</label>
                <select id="currency" [(ngModel)]="formData.currency" name="currency">
                  <option value="" disabled>Select currency</option>
                  <option value="USD">USD</option>
                  <option value="EUR">EUR</option>
                  <option value="GBP">GBP</option>
                </select>
              </div>
            </div>
            <div class="form-actions">
              <a routerLink="/merchants" class="btn btn-secondary">Cancel</a>
              <button type="submit" class="btn btn-primary" [disabled]="submitting || editForm.invalid">
                {{ submitting ? 'Saving...' : 'Save Changes' }}
              </button>
            </div>
          </form>
        </div>
      </ng-container>
    </div>
  `,
  styles: [`
    .merchant-edit { padding: var(--od-spacing-xl); }
    .page-header { margin-bottom: var(--od-spacing-lg); }
    .back-link { color: var(--od-accent); text-decoration: none; font-size: 14px; display: inline-block; margin-bottom: 8px; }
    .back-link:hover { text-decoration: underline; }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary); }
    .loading {
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
    .error-banner {
      background: var(--od-bg-danger, #fef2f2);
      border: 1px solid var(--od-border-danger, #fecaca);
      border-radius: var(--od-radius);
      padding: 12px 16px;
      color: var(--od-danger, #991b1b);
      font-size: 14px;
      margin-bottom: 16px;
    }
    .form-card {
      background: var(--od-card-bg);
      border-radius: var(--od-radius);
      padding: 24px;
      box-shadow: var(--od-card-shadow);
      max-width: 640px;
    }
    .form-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }
    .form-group { display: flex; flex-direction: column; }
    label { font-size: 13px; font-weight: 500; color: var(--od-text-primary); margin-bottom: 6px; }
    input, select {
      padding: 8px 12px;
      border: 1px solid var(--od-border);
      border-radius: var(--od-radius-sm);
      font-size: 14px;
      color: var(--od-text-primary);
      background: var(--od-bg-primary);
      outline: none;
      transition: border-color 0.2s;
    }
    input:focus, select:focus { border-color: var(--od-accent); }
    input.ng-invalid.ng-touched, select.ng-invalid.ng-touched { border-color: var(--od-danger, #ef4444); }
    .field-error { color: var(--od-danger, #ef4444); font-size: 12px; margin-top: 4px; }
    .form-actions {
      display: flex;
      justify-content: flex-end;
      gap: 12px;
      margin-top: 24px;
      padding-top: 16px;
      border-top: 1px solid var(--od-border);
    }
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
    .btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-secondary { background: var(--od-bg-secondary); color: var(--od-text-primary); display: inline-flex; align-items: center; }
    .btn-secondary:hover { background: var(--od-bg-tertiary); }
  `]
})
export class MerchantEditComponent implements OnInit {
  formData = {
    legal_name: '',
    email: '',
    trade_name: '',
    phone: '',
    country: '',
    currency: ''
  };
  submitting = false;
  loaded = false;
  error = '';
  private merchantId = '';

  constructor(private api: ApiService, private router: Router, private route: ActivatedRoute) {}

  ngOnInit(): void {
    this.merchantId = this.route.snapshot.paramMap.get('id')!;
    if (!this.merchantId) {
      this.router.navigate(['/merchants']);
      return;
    }
    this.api.getMerchant(this.merchantId).subscribe({
      next: (merchant) => {
        this.formData = {
          legal_name: (merchant as any).legal_name || merchant.name || '',
          email: merchant.email || '',
          trade_name: (merchant as any).trade_name || '',
          phone: (merchant as any).phone || '',
          country: (merchant as any).country || '',
          currency: (merchant as any).currency || ''
        };
        this.loaded = true;
      },
      error: () => this.router.navigate(['/merchants'])
    });
  }

  onSubmit(): void {
    if (!this.formData.legal_name || !this.formData.email) return;
    this.submitting = true;
    this.error = '';
    this.api.updateMerchant(this.merchantId, this.formData).subscribe({
      next: () => this.router.navigate(['/merchants', this.merchantId]),
      error: () => {
        this.error = 'Failed to update merchant. Please check your input and try again.';
        this.submitting = false;
      }
    });
  }
}
