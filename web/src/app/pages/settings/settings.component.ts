import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, ApiKey } from '../../core/api.service';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="page">
      <div class="page-header">
        <h1>Settings</h1>
      </div>

      <div class="loading" *ngIf="loading">
        <div class="spinner"></div>
        <span>Loading settings...</span>
      </div>

      <div class="error-banner" *ngIf="error">
        <span>{{ error }}</span>
        <button class="btn btn-sm" (click)="loadSettings()">Retry</button>
      </div>

      <div class="settings-grid" *ngIf="!loading && !error">
        <div class="card">
          <h2>Merchant Profile</h2>
          <form (ngSubmit)="saveProfile()">
            <div class="form-group">
              <label>Name</label>
              <input type="text" [(ngModel)]="profile.name" name="name" required />
            </div>
            <div class="form-group">
              <label>Email</label>
              <input type="email" [(ngModel)]="profile.email" name="email" required />
            </div>
            <div class="form-group">
              <label>Phone</label>
              <input type="tel" [(ngModel)]="profile.phone" name="phone" placeholder="+1 (555) 000-0000" />
            </div>
            <div class="form-actions">
              <button type="submit" class="btn btn-primary" [disabled]="saving">
                {{ saving ? 'Saving...' : 'Save Changes' }}
              </button>
            </div>
          </form>
          <div class="success-message" *ngIf="saveSuccess">Profile updated successfully.</div>
        </div>

        <div class="card">
          <div class="card-header">
            <h2>API Keys</h2>
            <button class="btn btn-primary" (click)="showApiKeyForm = !showApiKeyForm">
              {{ showApiKeyForm ? 'Cancel' : 'New Key' }}
            </button>
          </div>

          <div class="api-key-form" *ngIf="showApiKeyForm">
            <form (ngSubmit)="createApiKey()">
              <div class="form-group">
                <label>Key Name</label>
                <input type="text" [(ngModel)]="newKeyName" name="keyName" required
                  placeholder="e.g. Development" />
              </div>
              <div class="form-actions">
                <button type="submit" class="btn btn-primary" [disabled]="creatingKey || !newKeyName.trim()">
                  {{ creatingKey ? 'Creating...' : 'Create Key' }}
                </button>
              </div>
            </form>
            <div class="new-key-display" *ngIf="newlyCreatedKey">
              <label>Your API Key (copy now — it won't be shown again)</label>
              <div class="key-value">{{ newlyCreatedKey }}</div>
            </div>
          </div>

          <div class="api-keys-list" *ngIf="apiKeys.length > 0; else noKeys">
            <div class="api-key-row" *ngFor="let key of apiKeys">
              <div class="key-info">
                <span class="key-name">{{ key.name }}</span>
                <span class="key-prefix">{{ key.prefix }}... | {{ key.created_at | date:'mediumDate' }}</span>
              </div>
              <div class="key-status">
                <span class="badge" [ngClass]="key.status">{{ key.status }}</span>
              </div>
              <button class="btn btn-sm btn-danger" *ngIf="key.status === 'active'"
                (click)="revokeApiKey(key)">Revoke</button>
            </div>
          </div>
          <ng-template #noKeys>
            <div class="empty-state">No API keys created yet.</div>
          </ng-template>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .page { padding: var(--od-spacing-xl, 24px); }
    .page-header { margin-bottom: var(--od-spacing-lg, 20px); }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary, #1a1a1a); }
    .btn {
      padding: 8px 16px; border: none; border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; cursor: pointer; font-family: var(--od-font-sans, inherit);
    }
    .btn:disabled { opacity: 0.6; cursor: not-allowed; }
    .btn-primary { background: var(--od-brand-primary, #4f46e5); color: #fff; }
    .btn-primary:hover { opacity: 0.9; }
    .btn-danger { background: var(--od-danger, #dc3545); color: #fff; }
    .btn-danger:hover { opacity: 0.9; }
    .btn-sm { padding: 4px 10px; font-size: 12px; }
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
    .success-message {
      margin-top: 12px; padding: 8px 12px; background: #dcfce7; color: #166534;
      border-radius: var(--od-radius-sm, 4px); font-size: 13px;
    }
    .settings-grid { display: grid; gap: var(--od-spacing-lg, 20px); }
    .card {
      background: var(--od-card-bg, #fff); border-radius: var(--od-radius, 8px);
      padding: var(--od-spacing-lg, 24px); box-shadow: var(--od-card-shadow, 0 1px 3px rgba(0,0,0,0.1));
    }
    .card h2 { margin: 0 0 var(--od-spacing-md, 16px); font-size: 18px; color: var(--od-text-primary, #333); }
    .card-header {
      display: flex; justify-content: space-between; align-items: center; margin-bottom: var(--od-spacing-md, 16px);
    }
    .card-header h2 { margin: 0; }
    .form-group { margin-bottom: var(--od-spacing-md, 16px); }
    .form-group label { display: block; margin-bottom: 6px; font-size: 14px; font-weight: 500; color: var(--od-text-primary, #333); }
    .form-group input {
      width: 100%; padding: 8px 12px; border: 1px solid var(--od-border, #ddd); border-radius: var(--od-radius-sm, 6px);
      font-size: 14px; box-sizing: border-box; font-family: var(--od-font-sans, inherit);
    }
    .form-group input:focus {
      outline: none; border-color: var(--od-brand-primary, #4f46e5); box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
    }
    .form-actions { display: flex; justify-content: flex-end; }
    .api-key-form {
      margin-bottom: var(--od-spacing-md, 16px); padding: var(--od-spacing-md, 16px);
      background: var(--od-bg-secondary, #f8f9fa); border-radius: var(--od-radius-sm, 6px);
    }
    .new-key-display { margin-top: 12px; }
    .new-key-display label { display: block; font-size: 13px; font-weight: 600; color: var(--od-danger, #dc3545); margin-bottom: 4px; }
    .key-value {
      padding: 8px 12px; background: #fff; border: 1px solid var(--od-border, #ddd);
      border-radius: var(--od-radius-sm, 4px); font-family: var(--od-font-mono, monospace);
      font-size: 13px; word-break: break-all; color: var(--od-text-primary, #333);
    }
    .api-keys-list { border-top: 1px solid var(--od-border, #eee); }
    .api-key-row {
      display: flex; align-items: center; gap: 12px; padding: 12px 0;
      border-bottom: 1px solid var(--od-bg-tertiary, #f3f4f6);
    }
    .api-key-row:last-child { border-bottom: none; }
    .key-info { flex: 1; }
    .key-name { display: block; font-size: 14px; font-weight: 500; color: var(--od-text-primary, #333); }
    .key-prefix { font-size: 12px; color: var(--od-text-muted, #999); }
    .badge {
      display: inline-block; padding: 2px 10px; border-radius: 12px;
      font-size: 12px; font-weight: 500; text-transform: capitalize;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.revoked { background: #fee2e2; color: #991b1b; }
    .empty-state { padding: 20px; text-align: center; color: var(--od-text-muted, #999); }
  `]
})
export class SettingsComponent implements OnInit {
  loading = true;
  error = '';
  saving = false;
  saveSuccess = false;
  showApiKeyForm = false;
  creatingKey = false;
  newKeyName = '';
  newlyCreatedKey = '';

  profile = { name: '', email: '', phone: '' };
  apiKeys: ApiKey[] = [];

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadSettings();
  }

  loadSettings(): void {
    this.loading = true;
    this.error = '';
    this.api.getSettings().subscribe({
      next: (data) => {
        const d = data as any;
        this.profile = { name: d.name || '', email: d.email || '', phone: d.phone || '' };
        this.loading = false;
      },
      error: () => { this.error = 'Failed to load settings.'; this.loading = false; },
    });
    this.api.getApiKeys().subscribe({
      next: (keys) => this.apiKeys = keys,
      error: () => {},
    });
  }

  saveProfile(): void {
    this.saving = true;
    this.saveSuccess = false;
    this.api.updateSettings({ ...this.profile }).subscribe({
      next: () => { this.saving = false; this.saveSuccess = true; },
      error: () => { this.error = 'Failed to save profile.'; this.saving = false; },
    });
  }

  createApiKey(): void {
    if (!this.newKeyName.trim()) return;
    this.creatingKey = true;
    this.newlyCreatedKey = '';
    this.api.createApiKey({ name: this.newKeyName.trim() }).subscribe({
      next: (key) => {
        this.newlyCreatedKey = key.key || '';
        this.newKeyName = '';
        this.creatingKey = false;
        this.showApiKeyForm = false;
        this.api.getApiKeys().subscribe(keys => this.apiKeys = keys);
      },
      error: () => { this.error = 'Failed to create API key.'; this.creatingKey = false; },
    });
  }

  revokeApiKey(key: ApiKey): void {
    if (!confirm(`Revoke API key "${key.name}"? This cannot be undone.`)) return;
    this.api.revokeApiKey(key.id).subscribe({
      next: () => this.api.getApiKeys().subscribe(keys => this.apiKeys = keys),
      error: () => this.error = 'Failed to revoke API key.',
    });
  }
}
