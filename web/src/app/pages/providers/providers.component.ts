import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, ProviderConfig } from '../../core/api.service';

@Component({
  selector: 'app-providers',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="providers-page">
      <div class="page-header">
        <h1>Payment Providers</h1>
        <button class="btn-primary" (click)="showForm = !showForm">
          {{ showForm ? 'Cancel' : 'Add Provider' }}
        </button>
      </div>

      <div class="form-card" *ngIf="showForm">
        <h3>Add Provider Configuration</h3>
        <form (ngSubmit)="onSubmit()">
          <div class="form-row">
            <div class="form-group">
              <label>Provider</label>
              <select [(ngModel)]="newProvider.provider" name="provider" required>
                <option value="">Select provider</option>
                <option value="stripe">Stripe</option>
                <option value="paypal">PayPal</option>
                <option value="square">Square</option>
              </select>
            </div>
            <div class="form-group">
              <label>API Key</label>
              <input type="password" [(ngModel)]="apiKey" name="apiKey" required placeholder="sk_..." />
            </div>
          </div>
          <div class="form-group">
            <label>Webhook Secret (optional)</label>
            <input type="password" [(ngModel)]="webhookSecret" name="webhookSecret" placeholder="whsec_..." />
          </div>
          <div class="form-actions">
            <button type="submit" class="btn-primary" [disabled]="submitting">
              {{ submitting ? 'Saving...' : 'Save Configuration' }}
            </button>
          </div>
        </form>
      </div>

      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Provider</th>
              <th>Status</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let p of providers">
              <td class="provider-name">
                <span class="provider-icon" [ngClass]="p.provider">{{ p.provider | titlecase }}</span>
              </td>
              <td>
                <span class="badge" [ngClass]="p.status">{{ p.status }}</span>
              </td>
              <td>{{ p.created_at | date:'mediumDate' }}</td>
              <td>
                <button class="btn-danger-sm" (click)="onDelete(p.id)">Delete</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="providers.length === 0 && !loading">No providers configured.</div>
      </div>
    </div>
  `,
  styles: [`
    .providers-page { padding: 24px; }
    .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .btn-primary {
      padding: 8px 16px; background: #4f46e5; color: white; border: none;
      border-radius: 6px; font-size: 14px; cursor: pointer;
    }
    .btn-primary:hover { background: #4338ca; }
    .btn-primary:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-danger-sm {
      padding: 4px 10px; background: #fee2e2; color: #991b1b; border: none;
      border-radius: 4px; font-size: 12px; cursor: pointer;
    }
    .btn-danger-sm:hover { background: #fecaca; }
    .form-card {
      background: white; border-radius: 8px; padding: 24px; margin-bottom: 20px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
    }
    .form-card h3 { margin: 0 0 16px; font-size: 16px; color: #333; }
    .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .form-group { margin-bottom: 16px; }
    .form-group label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; color: #333; }
    .form-group input, .form-group select {
      width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px;
      font-size: 14px; box-sizing: border-box;
    }
    .form-group input:focus, .form-group select:focus {
      outline: none; border-color: #4f46e5; box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
    }
    .form-actions { display: flex; justify-content: flex-end; }
    .table-container {
      background: white; border-radius: 8px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; }
    th { background: #f9fafb; color: #666; font-weight: 500; border-bottom: 1px solid #e5e7eb; }
    td { border-bottom: 1px solid #f3f4f6; color: #333; }
    tr:last-child td { border-bottom: none; }
    .provider-name { font-weight: 500; }
    .provider-icon {
      display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: 500;
    }
    .provider-icon.stripe { background: #e8eaf6; color: #283593; }
    .provider-icon.paypal { background: #e3f2fd; color: #1565c0; }
    .provider-icon.square { background: #f3e5f5; color: #6a1b9a; }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.inactive { background: #f3f4f6; color: #6b7280; }
    .badge.error { background: #fee2e2; color: #991b1b; }
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class ProvidersComponent implements OnInit {
  providers: ProviderConfig[] = [];
  loading = true;
  showForm = false;
  submitting = false;
  apiKey = '';
  webhookSecret = '';
  newProvider: Partial<ProviderConfig> = { provider: '' };

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadProviders();
  }

  loadProviders(): void {
    this.api.getProviders('default').subscribe({
      next: (res) => { this.providers = res.data; this.loading = false; },
      error: () => this.loading = false,
    });
  }

  onSubmit(): void {
    this.submitting = true;
    const config: Record<string, unknown> = { api_key: this.apiKey };
    if (this.webhookSecret) config['webhook_secret'] = this.webhookSecret;
    this.newProvider.config = config;

    this.api.createProvider('default', this.newProvider).subscribe({
      next: () => {
        this.showForm = false;
        this.newProvider = { provider: '' };
        this.apiKey = '';
        this.webhookSecret = '';
        this.loadProviders();
        this.submitting = false;
      },
      error: () => this.submitting = false,
    });
  }

  onDelete(id: string): void {
    if (!confirm('Delete this provider configuration?')) return;
    this.api.deleteProvider('default', id).subscribe({ next: () => this.loadProviders() });
  }
}
