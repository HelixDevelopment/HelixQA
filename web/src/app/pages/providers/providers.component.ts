import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, ProviderConfig } from '../../core/api.service';
import { PageHeaderComponent, StatusBadgeComponent } from '../../shared/index';

@Component({
  selector: 'app-providers',
  standalone: true,
  imports: [CommonModule, FormsModule, PageHeaderComponent, StatusBadgeComponent],
  template: `
    <div class="page">
      <app-page-header title="Payment Providers">
        <button class="btn btn-primary" (click)="toggleForm()">
          {{ showForm ? 'Cancel' : 'Add Provider' }}
        </button>
      </app-page-header>

      <div class="loading" *ngIf="loading">
        <div class="spinner"></div>
        <span>Loading providers...</span>
      </div>

      <div class="error-banner" *ngIf="error">
        <span>{{ error }}</span>
        <button class="btn btn-sm" (click)="loadProviders()">Retry</button>
      </div>

      <div class="form-card" *ngIf="showForm">
        <h3>{{ editingId ? 'Update' : 'Add' }} Provider Configuration</h3>
        <form (ngSubmit)="onSubmit()">
          <div class="form-row">
            <div class="form-group">
              <label>Provider</label>
              <select [(ngModel)]="formProvider" name="provider" required>
                <option value="">Select provider</option>
                <option value="stripe">Stripe</option>
                <option value="paypal">PayPal</option>
                <option value="square">Square</option>
              </select>
            </div>
            <div class="form-group">
              <label>API Key</label>
              <input type="password" [(ngModel)]="formApiKey" name="apiKey" required placeholder="sk_..." />
            </div>
          </div>
          <div class="form-group">
            <label>Webhook Secret (optional)</label>
            <input type="password" [(ngModel)]="formSecret" name="webhookSecret" placeholder="whsec_..." />
          </div>
          <div class="form-actions">
            <button type="submit" class="btn btn-primary" [disabled]="submitting">
              {{ submitting ? 'Saving...' : 'Save Configuration' }}
            </button>
          </div>
        </form>
      </div>

      <div class="table-container" *ngIf="!loading && !error">
        <table *ngIf="providers.length > 0">
          <thead>
            <tr>
              <th>Provider</th>
              <th>Status</th>
              <th>Health</th>
              <th>Created</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let p of providers">
              <td class="provider-name">
                <span class="provider-badge" [ngClass]="p.provider">{{ p.provider | titlecase }}</span>
              </td>
              <td>
                <app-status-badge [label]="p.status" [variant]="p.status"></app-status-badge>
              </td>
              <td>
                <app-status-badge [label]="p.health || 'unknown'" [variant]="p.health === 'healthy' ? 'healthy' : 'unhealthy'"></app-status-badge>
              </td>
              <td>{{ p.created_at | date:'mediumDate' }}</td>
              <td class="actions-cell">
                <button class="btn btn-sm" [ngClass]="p.status === 'active' ? 'btn-warning' : 'btn-success'"
                  (click)="toggleStatus(p)">
                  {{ p.status === 'active' ? 'Deactivate' : 'Activate' }}
                </button>
                <button class="btn btn-sm btn-secondary" (click)="editProvider(p)">Configure</button>
                <button class="btn btn-sm btn-danger" (click)="onDelete(p.id)">Delete</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="providers.length === 0">No providers configured yet.</div>
      </div>
    </div>
  `,
  styles: [`
    .page { padding: var(--od-spacing-xl, 24px); }
    .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--od-spacing-lg, 20px); }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary, #1a1a1a); }
    .btn {
      padding: 8px 16px; border: none; border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; cursor: pointer; font-family: var(--od-font-sans, inherit);
    }
    .btn:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-primary { background: var(--od-brand-primary, #4f46e5); color: #fff; }
    .btn-primary:hover { opacity: 0.9; }
    .btn-secondary { background: var(--od-bg-tertiary, #e9ecef); color: var(--od-text-primary, #333); }
    .btn-secondary:hover { opacity: 0.85; }
    .btn-success { background: var(--od-success, #28a745); color: #fff; }
    .btn-success:hover { opacity: 0.9; }
    .btn-warning { background: var(--od-warning, #ffc107); color: #212529; }
    .btn-warning:hover { opacity: 0.9; }
    .btn-danger { background: var(--od-danger, #dc3545); color: #fff; }
    .btn-danger:hover { opacity: 0.9; }
    .btn-sm { padding: 4px 10px; font-size: 12px; margin-right: 4px; }
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
    .form-card {
      background: var(--od-card-bg, #fff); border-radius: var(--od-radius, 8px); padding: var(--od-spacing-lg, 24px);
      margin-bottom: var(--od-spacing-lg, 20px); box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
    }
    .form-card h3 { margin: 0 0 var(--od-spacing-md, 16px); font-size: 16px; color: var(--od-text-primary, #333); }
    .form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .form-group { margin-bottom: var(--od-spacing-md, 16px); }
    .form-group label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; color: var(--od-text-primary, #333); }
    .form-group input, .form-group select {
      width: 100%; padding: 8px 12px; border: 1px solid var(--od-border, #ddd); border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; box-sizing: border-box; font-family: var(--od-font-sans, inherit);
    }
    .form-group input:focus, .form-group select:focus {
      outline: none; border-color: var(--od-brand-primary, #4f46e5); box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
    }
    .form-actions { display: flex; justify-content: flex-end; }
    .table-container {
      background: var(--od-card-bg, #fff); border-radius: var(--od-radius, 8px);
      box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1)); overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; }
    th {
      background: var(--od-bg-secondary, #f9fafb); color: var(--od-text-secondary, #666);
      font-weight: 500; border-bottom: 1px solid var(--od-border, #e5e7eb);
    }
    td { border-bottom: 1px solid var(--od-bg-tertiary, #f3f4f6); color: var(--od-text-primary, #333); }
    tr:last-child td { border-bottom: none; }
    .actions-cell { white-space: nowrap; }
    .provider-name { font-weight: 500; }
    .provider-badge {
      display: inline-block; padding: 2px 8px; border-radius: var(--od-radius-sm, 4px);
      font-size: 12px; font-weight: 500;
    }
    .provider-badge.stripe { background: #e8eaf6; color: #283593; }
    .provider-badge.paypal { background: #e3f2fd; color: #1565c0; }
    .provider-badge.square { background: #f3e5f5; color: #6a1b9a; }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.inactive { background: #f3f4f6; color: #6b7280; }
    .badge.error { background: #fee2e2; color: #991b1b; }
    .badge-health.healthy { background: #dcfce7; color: #166534; }
    .badge-health.degraded { background: #fef9c3; color: #854d0e; }
    .badge-health.unhealthy { background: #fee2e2; color: #991b1b; }
    .badge-health.unknown { background: #f3f4f6; color: #6b7280; }
    .empty-state { padding: 40px; text-align: center; color: var(--od-text-muted, #999); }
  `]
})
export class ProvidersComponent implements OnInit {
  providers: ProviderConfig[] = [];
  loading = true;
  error = '';
  showForm = false;
  submitting = false;
  editingId = '';
  formProvider = '';
  formApiKey = '';
  formSecret = '';
  private merchantId = '';

  constructor(private api: ApiService) {
    this.merchantId = localStorage.getItem('helix_merchant_id') || '';
  }

  ngOnInit(): void {
    this.loadProviders();
  }

  loadProviders(): void {
    this.loading = true;
    this.error = '';
    if (!this.merchantId) { this.error = 'No merchant selected.'; this.loading = false; return; }
    this.api.getProviders(this.merchantId).subscribe({
      next: (res) => { this.providers = res.data; this.loading = false; },
      error: () => { this.error = 'Failed to load providers.'; this.loading = false; },
    });
  }

  toggleForm(): void {
    this.showForm = !this.showForm;
    if (!this.showForm) this.resetForm();
  }

  editProvider(p: ProviderConfig): void {
    this.editingId = p.id;
    this.formProvider = p.provider;
    this.formApiKey = (p.config?.['api_key'] as string) || '';
    this.formSecret = (p.config?.['webhook_secret'] as string) || '';
    this.showForm = true;
  }

  resetForm(): void {
    this.editingId = '';
    this.formProvider = '';
    this.formApiKey = '';
    this.formSecret = '';
  }

  onSubmit(): void {
    this.submitting = true;
    const config: Record<string, unknown> = { api_key: this.formApiKey };
    if (this.formSecret) config['webhook_secret'] = this.formSecret;
    const payload: Partial<ProviderConfig> = { provider: this.formProvider, config };

    const request = this.editingId
      ? this.api.updateProvider(this.merchantId, this.editingId, payload)
      : this.api.createProvider(this.merchantId, payload);

    request.subscribe({
      next: () => {
        this.showForm = false;
        this.resetForm();
        this.loadProviders();
        this.submitting = false;
      },
      error: () => { this.error = 'Failed to save provider.'; this.submitting = false; },
    });
  }

  toggleStatus(p: ProviderConfig): void {
    const newStatus = p.status === 'active' ? 'inactive' : 'active';
    this.api.updateProvider(this.merchantId, p.id, { status: newStatus } as any).subscribe({
      next: () => this.loadProviders(),
      error: () => this.error = 'Failed to update provider status.',
    });
  }

  onDelete(id: string): void {
    if (!confirm('Delete this provider configuration?')) return;
    this.api.deleteProvider(this.merchantId, id).subscribe({
      next: () => this.loadProviders(),
      error: () => this.error = 'Failed to delete provider.',
    });
  }
}
