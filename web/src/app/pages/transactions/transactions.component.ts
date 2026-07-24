import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-transactions',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="transactions-page">
      <div class="page-header">
        <h1>Transactions</h1>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Merchant</th>
              <th>Type</th>
              <th>Amount</th>
              <th>Currency</th>
              <th>Provider</th>
              <th>Status</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let tx of transactions">
              <td class="id-cell">{{ tx.id }}</td>
              <td>
                <a [routerLink]="['/merchants', tx.merchant_id]">{{ tx.merchant_id }}</a>
              </td>
              <td>{{ tx.type }}</td>
              <td class="amount-cell">\${{ tx.amount / 100 | number:'1.2-2' }}</td>
              <td>{{ tx.currency }}</td>
              <td>{{ tx.provider }}</td>
              <td>
                <span class="badge" [ngClass]="tx.status">{{ tx.status }}</span>
              </td>
              <td>{{ tx.created_at | date:'medium' }}</td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="transactions.length === 0 && !loading">No transactions found.</div>
      </div>
    </div>
  `,
  styles: [`
    .transactions-page { padding: 24px; }
    .page-header { margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .table-container {
      background: white;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      overflow-x: auto;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; white-space: nowrap; }
    th { background: #f9fafb; color: #666; font-weight: 500; border-bottom: 1px solid #e5e7eb; }
    td { border-bottom: 1px solid #f3f4f6; color: #333; }
    tr:last-child td { border-bottom: none; }
    .id-cell { font-family: monospace; font-size: 12px; max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
    .amount-cell { font-weight: 500; }
    td a { color: #4f46e5; text-decoration: none; }
    td a:hover { text-decoration: underline; }
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
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class TransactionsComponent implements OnInit {
  transactions: any[] = [];
  loading = true;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getTransactions().subscribe({
      next: (res) => {
        this.transactions = res.data;
        this.loading = false;
      },
      error: () => this.loading = false
    });
  }
}
