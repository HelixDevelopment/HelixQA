import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink, Router } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-merchant-create',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  template: `
    <div class="merchant-create">
      <div class="page-header">
        <a routerLink="/merchants" class="back-link">&larr; Back to Merchants</a>
        <h1>Create Merchant</h1>
      </div>
      <div class="form-card">
        <form (ngSubmit)="onSubmit()">
          <div class="form-grid">
            <div class="form-group">
              <label for="name">Legal Name</label>
              <input id="name" type="text" [(ngModel)]="formData.name" name="name" required>
            </div>
            <div class="form-group">
              <label for="email">Email</label>
              <input id="email" type="email" [(ngModel)]="formData.email" name="email" required>
            </div>
            <div class="form-group">
              <label for="trade_name">Trade Name</label>
              <input id="trade_name" type="text" [(ngModel)]="formData.trade_name" name="trade_name">
            </div>
            <div class="form-group">
              <label for="country">Country</label>
              <select id="country" [(ngModel)]="formData.country" name="country" required>
                <option value="" disabled>Select country</option>
                <option value="US">United States</option>
                <option value="GB">United Kingdom</option>
                <option value="DE">Germany</option>
                <option value="FR">France</option>
              </select>
            </div>
            <div class="form-group">
              <label for="currency">Currency</label>
              <select id="currency" [(ngModel)]="formData.currency" name="currency" required>
                <option value="" disabled>Select currency</option>
                <option value="USD">USD</option>
                <option value="EUR">EUR</option>
                <option value="GBP">GBP</option>
              </select>
            </div>
          </div>
          <div class="form-actions">
            <a routerLink="/merchants" class="btn btn-secondary">Cancel</a>
            <button type="submit" class="btn btn-primary" [disabled]="submitting">
              {{ submitting ? 'Creating...' : 'Create Merchant' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  `,
  styles: [`
    .merchant-create { padding: 24px; }
    .page-header { margin-bottom: 24px; }
    .back-link { color: #4f46e5; text-decoration: none; font-size: 14px; display: inline-block; margin-bottom: 8px; }
    .back-link:hover { text-decoration: underline; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .form-card {
      background: white;
      border-radius: 8px;
      padding: 24px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      max-width: 640px;
    }
    .form-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 16px;
    }
    .form-group { display: flex; flex-direction: column; }
    .form-group.full-width { grid-column: 1 / -1; }
    label { font-size: 13px; font-weight: 500; color: #333; margin-bottom: 6px; }
    input, select {
      padding: 8px 12px;
      border: 1px solid #d1d5db;
      border-radius: 6px;
      font-size: 14px;
      color: #1a1a1a;
      background: white;
      outline: none;
      transition: border-color 0.2s;
    }
    input:focus, select:focus { border-color: #4f46e5; }
    .form-actions {
      display: flex;
      justify-content: flex-end;
      gap: 12px;
      margin-top: 24px;
      padding-top: 16px;
      border-top: 1px solid #f3f4f6;
    }
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
    .btn-primary:disabled { background: #a5b4fc; cursor: not-allowed; }
    .btn-secondary { background: #f3f4f6; color: #374151; display: inline-flex; align-items: center; }
    .btn-secondary:hover { background: #e5e7eb; }
  `]
})
export class MerchantCreateComponent {
  formData = {
    name: '',
    email: '',
    trade_name: '',
    country: '',
    currency: ''
  };
  submitting = false;

  constructor(private api: ApiService, private router: Router) {}

  onSubmit(): void {
    this.submitting = true;
    this.api.createMerchant(this.formData).subscribe({
      next: () => this.router.navigate(['/merchants']),
      error: () => this.submitting = false
    });
  }
}
