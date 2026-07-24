import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="page">
      <div class="page-header">
        <h1>Transactions</h1>
      </div>

      <div class="filters">
        <select [(ngModel)]="statusFilter" (change)="onFilterChange()">
          <option value="">All Statuses</option>
          <option value="succeeded">Completed</option>
          <option value="pending">Pending</option>
          <option value="failed">Failed</option>
          <option value="processing">Processing</option>
          <option value="cancelled">Cancelled</option>
          <option value="reversed">Reversed</option>
        </select>
      </div>

      <div class="loading-overlay" *ngIf="loading">
        <div class="spinner"></div>
      </div>

      <div class="error-state" *ngIf="error">
        <p>Failed to load transactions.</p>
        <button class="btn" (click)="loadTransactions()">Retry</button>
      </div>

      <ng-container *ngIf="!loading && !error">
        <div class="table-container">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Amount</th>
                <th>Currency</th>
                <th>Status</th>
                <th>Provider</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <ng-container *ngFor="let tx of filteredTransactions; let i = index">
                <tr class="clickable" (click)="toggleRow(i)">
                  <td class="id-cell">{{ tx.id }}</td>
                  <td class="amount-cell">{{ tx.amount / 100 | number:'1.2-2' }}</td>
                  <td>{{ tx.currency }}</td>
                  <td>
                    <span class="badge" [ngClass]="tx.status">{{ statusLabel(tx.status) }}</span>
                  </td>
                  <td>{{ tx.provider }}</td>
                  <td>{{ tx.created_at | date:'medium' }}</td>
                </tr>
                <tr class="detail-row" *ngIf="expandedRow === i">
                  <td colspan="6">
                    <div class="detail-grid">
                      <div class="detail-item">
                        <span class="detail-label">Transaction ID</span>
                        <span class="detail-value">{{ tx.id }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Amount</span>
                        <span class="detail-value">{{ tx.currency }} {{ tx.amount / 100 | number:'1.2-2' }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Status</span>
                        <span class="detail-value"><span class="badge" [ngClass]="tx.status">{{ statusLabel(tx.status) }}</span></span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Provider</span>
                        <span class="detail-value">{{ tx.provider }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Type</span>
                        <span class="detail-value">{{ tx.type }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Merchant ID</span>
                        <span class="detail-value">{{ tx.merchant_id }}</span>
                      </div>
                      <div class="detail-item">
                        <span class="detail-label">Created</span>
                        <span class="detail-value">{{ tx.created_at | date:'full' }}</span>
                      </div>
                    </div>
                  </td>
                </tr>
              </ng-container>
            </tbody>
          </table>

          <div class="empty-state" *ngIf="filteredTransactions.length === 0">No transactions found.</div>
        </div>

        <div class="pagination" *ngIf="totalPages > 1">
          <button class="btn btn-prev" [disabled]="page <= 1" (click)="goToPage(page - 1)">Previous</button>
          <span class="page-info">Page {{ page }} of {{ totalPages }}</span>
          <button class="btn btn-next" [disabled]="page >= totalPages" (click)="goToPage(page + 1)">Next</button>
        </div>
      </ng-container>
    </div>
  `,
  styles: [`
    .page { padding: var(--od-spacing-xl); }
    .page-header { margin-bottom: var(--od-spacing-lg); }
    h1 { margin: 0; font-size: 24px; color: var(--od-text-primary); font-family: var(--od-font-sans); }

    .filters { margin-bottom: var(--od-spacing-lg); }
    select {
      padding: 8px 12px;
      border: 1px solid var(--od-border);
      border-radius: var(--od-radius-sm);
      background: var(--od-bg-primary);
      color: var(--od-text-primary);
      font-size: 14px;
      font-family: var(--od-font-sans);
      min-width: 180px;
    }

    .loading-overlay {
      display: flex;
      justify-content: center;
      padding: 60px 0;
    }
    .spinner {
      width: 32px; height: 32px;
      border: 3px solid var(--od-border);
      border-top-color: var(--od-accent);
      border-radius: 50%;
      animation: spin 0.7s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }

    .error-state {
      text-align: center;
      padding: 60px 0;
      color: var(--od-text-secondary);
    }
    .error-state p { margin: 0 0 var(--od-spacing-md); }
    .btn {
      padding: 8px 16px;
      border: 1px solid var(--od-border);
      border-radius: var(--od-radius-sm);
      background: var(--od-bg-primary);
      color: var(--od-text-primary);
      font-size: 14px;
      cursor: pointer;
      font-family: var(--od-font-sans);
    }
    .btn:hover { background: var(--od-bg-secondary); }
    .btn:disabled { opacity: 0.5; cursor: default; }

    .table-container {
      background: var(--od-card-bg);
      border-radius: var(--od-radius);
      box-shadow: var(--od-card-shadow);
      overflow-x: auto;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; white-space: nowrap; color: var(--od-text-primary); }
    th { background: var(--od-bg-secondary); color: var(--od-text-secondary); font-weight: 500; border-bottom: 1px solid var(--od-border); }
    td { border-bottom: 1px solid var(--od-border); }
    tr:last-child td { border-bottom: none; }

    tr.clickable { cursor: pointer; }
    tr.clickable:hover td { background: var(--od-bg-secondary); }

    .id-cell { font-family: var(--od-font-mono); font-size: 12px; max-width: 120px; overflow: hidden; text-overflow: ellipsis; color: var(--od-text-muted); }
    .amount-cell { font-weight: 500; }

    .badge {
      display: inline-block;
      padding: 2px 10px;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 500;
      text-transform: capitalize;
    }
    .badge.succeeded { background: #dcfce7; color: #166534; }
    .badge.pending { background: #fef9c3; color: #854d0e; }
    .badge.processing { background: #dbeafe; color: #1e40af; }
    .badge.failed { background: #fee2e2; color: #991b1b; }
    .badge.cancelled { background: #f3f4f6; color: #6b7280; }
    .badge.reversed { background: #fce7f3; color: #9d174d; }

    .detail-row td {
      padding: 0;
      background: var(--od-bg-secondary);
    }
    .detail-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: var(--od-spacing-md);
      padding: var(--od-spacing-lg);
    }
    .detail-item {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }
    .detail-label { font-size: 12px; color: var(--od-text-muted); text-transform: uppercase; letter-spacing: 0.5px; }
    .detail-value { font-size: 14px; color: var(--od-text-primary); }

    .empty-state { padding: 40px; text-align: center; color: var(--od-text-muted); }

    .pagination {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: var(--od-spacing-md);
      margin-top: var(--od-spacing-lg);
      padding: var(--od-spacing-md) 0;
    }
    .page-info { font-size: 14px; color: var(--od-text-secondary); }
  `]
})
export class TransactionsComponent implements OnInit {
  transactions: any[] = [];
  filteredTransactions: any[] = [];
  loading = true;
  error = false;
  page = 1;
  perPage = 20;
  total = 0;
  totalPages = 0;
  statusFilter = '';
  expandedRow: number | null = null;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadTransactions();
  }

  loadTransactions(): void {
    this.loading = true;
    this.error = false;
    this.api.getTransactions(this.page, this.perPage).subscribe({
      next: (res) => {
        this.transactions = res.data;
        this.total = res.total;
        this.totalPages = Math.ceil(res.total / res.per_page);
        this.filteredTransactions = this.statusFilter
          ? this.transactions.filter(tx => tx.status === this.statusFilter)
          : [...this.transactions];
        this.loading = false;
      },
      error: () => {
        this.loading = false;
        this.error = true;
      }
    });
  }

  onFilterChange(): void {
    this.page = 1;
    this.expandedRow = null;
    this.loadTransactions();
  }

  goToPage(p: number): void {
    this.page = p;
    this.expandedRow = null;
    this.loadTransactions();
  }

  toggleRow(i: number): void {
    this.expandedRow = this.expandedRow === i ? null : i;
  }

  statusLabel(status: string): string {
    const labels: Record<string, string> = {
      succeeded: 'Completed',
      pending: 'Pending',
      failed: 'Failed',
      processing: 'Processing',
      cancelled: 'Cancelled',
      reversed: 'Reversed'
    };
    return labels[status] || status;
  }
}
