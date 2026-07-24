import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, WebhookConfig } from '../../core/api.service';

@Component({
  selector: 'app-webhooks',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="webhooks-page">
      <div class="page-header">
        <h1>Webhook Configurations</h1>
        <button class="btn-primary" (click)="showForm = !showForm">
          {{ showForm ? 'Cancel' : 'Add Webhook' }}
        </button>
      </div>

      <div class="form-card" *ngIf="showForm">
        <h3>Add Webhook</h3>
        <form (ngSubmit)="onSubmit()">
          <div class="form-group">
            <label>Endpoint URL</label>
            <input type="url" [(ngModel)]="newWebhook.url" name="url" required
              placeholder="https://your-server.com/webhook" />
          </div>
          <div class="form-group">
            <label>Events (comma-separated)</label>
            <input type="text" [(ngModel)]="eventsInput" name="events" required
              placeholder="payment.succeeded, payment.failed, subscription.created" />
          </div>
          <div class="form-actions">
            <button type="submit" class="btn-primary" [disabled]="submitting">
              {{ submitting ? 'Saving...' : 'Save Webhook' }}
            </button>
          </div>
        </form>
      </div>

      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>URL</th>
              <th>Events</th>
              <th>Status</th>
              <th>Created</th>
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
                <span class="badge" [ngClass]="w.status">{{ w.status }}</span>
              </td>
              <td>{{ w.created_at | date:'mediumDate' }}</td>
              <td>
                <button class="btn-danger-sm" (click)="onDelete(w.id)">Delete</button>
              </td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="webhooks.length === 0 && !loading">No webhooks configured.</div>
      </div>
    </div>
  `,
  styles: [`
    .webhooks-page { padding: 24px; }
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
    .form-group { margin-bottom: 16px; }
    .form-group label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; color: #333; }
    .form-group input {
      width: 100%; padding: 8px 12px; border: 1px solid #ddd; border-radius: 6px;
      font-size: 14px; box-sizing: border-box;
    }
    .form-group input:focus {
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
    .url-cell { font-family: monospace; font-size: 12px; max-width: 250px; overflow: hidden; text-overflow: ellipsis; }
    .event-tag {
      display: inline-block; padding: 2px 6px; margin: 1px 2px; background: #f0f0f0;
      border-radius: 4px; font-size: 11px; color: #555;
    }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.inactive { background: #f3f4f6; color: #6b7280; }
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class WebhooksComponent implements OnInit {
  webhooks: WebhookConfig[] = [];
  loading = true;
  showForm = false;
  submitting = false;
  eventsInput = '';
  newWebhook: Partial<WebhookConfig> = {};

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadWebhooks();
  }

  loadWebhooks(): void {
    this.api.getWebhooks('default').subscribe({
      next: (res) => { this.webhooks = res.data; this.loading = false; },
      error: () => this.loading = false,
    });
  }

  onSubmit(): void {
    this.submitting = true;
    this.newWebhook.events = this.eventsInput.split(',').map(e => e.trim()).filter(e => e);

    this.api.createWebhook('default', this.newWebhook).subscribe({
      next: () => {
        this.showForm = false;
        this.newWebhook = {};
        this.eventsInput = '';
        this.loadWebhooks();
        this.submitting = false;
      },
      error: () => this.submitting = false,
    });
  }

  onDelete(id: string): void {
    if (!confirm('Delete this webhook configuration?')) return;
    this.api.deleteWebhook('default', id).subscribe({ next: () => this.loadWebhooks() });
  }
}
