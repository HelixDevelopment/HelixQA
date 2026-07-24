import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, WebhookConfig, WebhookDelivery } from '../../core/api.service';
import { PageHeaderComponent, StatusBadgeComponent } from '../../shared/index';

@Component({
  selector: 'app-webhooks',
  standalone: true,
  imports: [CommonModule, FormsModule, PageHeaderComponent, StatusBadgeComponent],
  template: `
    <div class="page">
      <app-page-header title="Webhook Configurations">
        <button class="btn btn-primary" (click)="toggleForm()">
          {{ showForm ? 'Cancel' : 'Add Webhook' }}
        </button>
      </app-page-header>

      <div class="loading" *ngIf="loading">
        <div class="spinner"></div>
        <span>Loading webhooks...</span>
      </div>

      <div class="error-banner" *ngIf="error">
        <span>{{ error }}</span>
        <button class="btn btn-sm" (click)="loadWebhooks()">Retry</button>
      </div>

      <div class="form-card" *ngIf="showForm">
        <h3>{{ editingId ? 'Update' : 'Add' }} Webhook</h3>
        <form (ngSubmit)="onSubmit()">
          <div class="form-group">
            <label>Endpoint URL</label>
            <input type="url" [(ngModel)]="formUrl" name="url" required
              placeholder="https://your-server.com/webhook" />
          </div>
          <div class="form-group">
            <label>Webhook Secret</label>
            <input type="password" [(ngModel)]="formSecret" name="secret" placeholder="whsec_..." />
          </div>
          <div class="form-group">
            <label>Events</label>
            <div class="checkbox-grid">
              <label class="checkbox-label" *ngFor="let ev of availableEvents">
                <input type="checkbox" [value]="ev" (change)="toggleEvent(ev)"
                  [checked]="formEvents.includes(ev)" />
                {{ ev }}
              </label>
            </div>
          </div>
          <div class="form-actions">
            <button type="submit" class="btn btn-primary" [disabled]="submitting || formEvents.length === 0">
              {{ submitting ? 'Saving...' : 'Save Webhook' }}
            </button>
          </div>
        </form>
      </div>

      <div class="table-container" *ngIf="!loading && !error">
        <table *ngIf="webhooks.length > 0">
          <thead>
            <tr>
              <th>URL</th>
              <th>Events</th>
              <th>Status</th>
              <th>Last Delivery</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let w of webhooks">
              <td class="url-cell">{{ w.url }}</td>
              <td>
                <span class="event-tag" *ngFor="let e of w.events">{{ e }}</span>
              </td>
              <td>
                <app-status-badge [label]="w.status" [variant]="w.status"></app-status-badge>
              </td>
              <td class="delivery-cell">
                <span *ngIf="deliveries[w.id] && deliveries[w.id].length > 0" class="delivery-info">
                  <app-status-badge [label]="deliveries[w.id][0].status" [variant]="deliveries[w.id][0].status"></app-status-badge>
                  <span class="delivery-time">{{ deliveries[w.id][0].delivered_at | date:'short' }}</span>
                </span>
                <span *ngIf="!deliveries[w.id] || deliveries[w.id].length === 0" class="text-muted">—</span>
              </td>
              <td class="actions-cell">
                <button class="btn btn-sm btn-secondary" (click)="testWebhook(w)"
                  [disabled]="testing === w.id">
                  {{ testing === w.id ? 'Testing...' : 'Test' }}
                </button>
                <button class="btn btn-sm btn-secondary" (click)="toggleDeliveryLog(w.id)">
                  {{ expandedLog === w.id ? 'Hide Log' : 'Log' }}
                </button>
                <button class="btn btn-sm btn-danger" (click)="onDelete(w.id)">Delete</button>
              </td>
            </tr>
            <tr *ngIf="expandedLog" class="log-row">
              <td colspan="5">
                <div class="delivery-log" *ngIf="deliveries[expandedLog]">
                  <h4>Delivery Log</h4>
                  <div class="log-entry" *ngFor="let d of deliveries[expandedLog]">
                    <div class="log-header">
                      <app-status-badge [label]="d.status" [variant]="d.status"></app-status-badge>
                      <span class="log-event">{{ d.event }}</span>
                      <span class="log-time">{{ d.delivered_at | date:'medium' }}</span>
                    </div>
                    <div class="log-detail" *ngIf="d.response_code">
                      Response: {{ d.response_code }} | Body: {{ d.response_body || '—' }}
                    </div>
                    <div class="log-detail error" *ngIf="d.error">{{ d.error }}</div>
                  </div>
                  <div class="empty-state" *ngIf="deliveries[expandedLog].length === 0">No deliveries yet.</div>
                </div>
                <div class="loading" *ngIf="!deliveries[expandedLog]">
                  <div class="spinner"></div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="webhooks.length === 0">No webhooks configured yet.</div>
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
    .text-muted { color: var(--od-text-muted, #999); }
    .form-card {
      background: var(--od-card-bg, #fff); border-radius: var(--od-radius, 8px); padding: var(--od-spacing-lg, 24px);
      margin-bottom: var(--od-spacing-lg, 20px); box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
    }
    .form-card h3 { margin: 0 0 var(--od-spacing-md, 16px); font-size: 16px; color: var(--od-text-primary, #333); }
    .form-group { margin-bottom: var(--od-spacing-md, 16px); }
    .form-group label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; color: var(--od-text-primary, #333); }
    .form-group input {
      width: 100%; padding: 8px 12px; border: 1px solid var(--od-border, #ddd); border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; box-sizing: border-box; font-family: var(--od-font-sans, inherit);
    }
    .form-group input:focus {
      outline: none; border-color: var(--od-brand-primary, #4f46e5); box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
    }
    .checkbox-grid { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 4px; }
    .checkbox-label {
      display: flex; align-items: center; gap: 4px; font-size: 13px; padding: 4px 8px;
      background: var(--od-bg-secondary, #f8f9fa); border-radius: var(--od-radius-sm, 4px);
      cursor: pointer; color: var(--od-text-primary, #333);
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
    .url-cell {
      font-family: var(--od-font-mono, monospace); font-size: 12px;
      max-width: 250px; overflow: hidden; text-overflow: ellipsis;
    }
    .event-tag {
      display: inline-block; padding: 2px 6px; margin: 1px 2px;
      background: var(--od-bg-tertiary, #f0f0f0); border-radius: var(--od-radius-sm, 4px);
      font-size: 11px; color: var(--od-text-secondary, #555);
    }
    .delivery-cell { font-size: 13px; }
    .delivery-info { display: flex; align-items: center; gap: 6px; }
    .delivery-time { font-size: 12px; color: var(--od-text-muted, #999); }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge-sm { padding: 1px 6px; font-size: 11px; }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.inactive { background: #f3f4f6; color: #6b7280; }
    .badge.success { background: #dcfce7; color: #166534; }
    .badge.failed { background: #fee2e2; color: #991b1b; }
    .badge.pending { background: #fef9c3; color: #854d0e; }
    .log-row td { padding: 0; }
    .log-row td { border-bottom: 1px solid var(--od-border, #e5e7eb); }
    .delivery-log { padding: 16px; background: var(--od-bg-secondary, #f9fafb); }
    .delivery-log h4 { margin: 0 0 12px; font-size: 14px; color: var(--od-text-primary, #333); }
    .log-entry {
      padding: 8px 12px; margin-bottom: 8px; background: var(--od-card-bg, #fff);
      border-radius: var(--od-radius-sm, 4px); border: 1px solid var(--od-border, #eee);
    }
    .log-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
    .log-event { font-size: 13px; font-weight: 500; color: var(--od-text-primary, #333); }
    .log-time { font-size: 11px; color: var(--od-text-muted, #999); margin-left: auto; }
    .log-detail { font-size: 12px; color: var(--od-text-secondary, #666); margin-top: 2px; }
    .log-detail.error { color: var(--od-danger, #dc3545); }
    .empty-state { padding: 40px; text-align: center; color: var(--od-text-muted, #999); }
  `]
})
export class WebhooksComponent implements OnInit {
  webhooks: WebhookConfig[] = [];
  deliveries: Record<string, WebhookDelivery[]> = {};
  loading = true;
  error = '';
  showForm = false;
  submitting = false;
  testing = '';
  expandedLog = '';
  editingId = '';
  formUrl = '';
  formSecret = '';
  formEvents: string[] = [];
  private merchantId = '';

  availableEvents = [
    'payment.succeeded', 'payment.failed', 'payment.refunded',
    'subscription.created', 'subscription.cancelled', 'subscription.updated',
    'charge.disputed', 'customer.created',
  ];

  constructor(private api: ApiService) {
    this.merchantId = localStorage.getItem('helix_merchant_id') || '';
  }

  ngOnInit(): void {
    this.loadWebhooks();
  }

  loadWebhooks(): void {
    this.loading = true;
    this.error = '';
    if (!this.merchantId) { this.error = 'No merchant selected.'; this.loading = false; return; }
    this.api.getWebhooks(this.merchantId).subscribe({
      next: (res) => {
        this.webhooks = res.data;
        this.loading = false;
        this.webhooks.forEach(w => this.loadDeliveries(w.id));
      },
      error: () => { this.error = 'Failed to load webhooks.'; this.loading = false; },
    });
  }

  loadDeliveries(id: string): void {
    this.api.getWebhookDeliveries(this.merchantId, id).subscribe({
      next: (res) => this.deliveries[id] = res,
      error: () => this.deliveries[id] = [],
    });
  }

  toggleForm(): void {
    this.showForm = !this.showForm;
    if (!this.showForm) this.resetForm();
  }

  toggleEvent(ev: string): void {
    const idx = this.formEvents.indexOf(ev);
    if (idx >= 0) this.formEvents.splice(idx, 1);
    else this.formEvents.push(ev);
  }

  resetForm(): void {
    this.editingId = '';
    this.formUrl = '';
    this.formSecret = '';
    this.formEvents = [];
  }

  onSubmit(): void {
    this.submitting = true;
    const payload: Partial<WebhookConfig> = {
      url: this.formUrl,
      secret: this.formSecret,
      events: this.formEvents,
    };

    const request = this.editingId
      ? this.api.updateWebhook(this.merchantId, this.editingId, payload)
      : this.api.createWebhook(this.merchantId, payload);

    request.subscribe({
      next: () => {
        this.showForm = false;
        this.resetForm();
        this.loadWebhooks();
        this.submitting = false;
      },
      error: () => { this.error = 'Failed to save webhook.'; this.submitting = false; },
    });
  }

  onDelete(id: string): void {
    if (!confirm('Delete this webhook configuration?')) return;
    this.api.deleteWebhook(this.merchantId, id).subscribe({
      next: () => this.loadWebhooks(),
      error: () => this.error = 'Failed to delete webhook.',
    });
  }

  testWebhook(w: WebhookConfig): void {
    this.testing = w.id;
    this.api.testWebhook(this.merchantId, w.id).subscribe({
      next: () => {
        this.loadDeliveries(w.id);
        this.testing = '';
      },
      error: () => { this.error = 'Test delivery failed.'; this.testing = ''; },
    });
  }

  toggleDeliveryLog(id: string): void {
    this.expandedLog = this.expandedLog === id ? '' : id;
    if (this.expandedLog && !this.deliveries[id]) {
      this.loadDeliveries(id);
    }
  }
}
