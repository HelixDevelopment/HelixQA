import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-merchants',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <div class="merchants-page">
      <div class="page-header">
        <h1>Merchants</h1>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Country</th>
              <th>Currency</th>
              <th>Status</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let merchant of merchants">
              <td>
                <a [routerLink]="['/merchants', merchant.id]">{{ merchant.trade_name || merchant.name }}</a>
              </td>
              <td>{{ merchant.email }}</td>
              <td>{{ merchant.country }}</td>
              <td>{{ merchant.currency }}</td>
              <td>
                <span class="badge" [ngClass]="merchant.status">
                  {{ merchant.status }}
                </span>
              </td>
              <td>{{ merchant.created_at | date:'mediumDate' }}</td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="merchants.length === 0 && !loading">No merchants found.</div>
      </div>
    </div>
  `,
  styles: [`
    .merchants-page { padding: 24px; }
    .page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
    h1 { margin: 0; font-size: 24px; color: #1a1a1a; }
    .table-container {
      background: white;
      border-radius: 8px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; }
    th { background: #f9fafb; color: #666; font-weight: 500; border-bottom: 1px solid #e5e7eb; }
    td { border-bottom: 1px solid #f3f4f6; color: #333; }
    tr:last-child td { border-bottom: none; }
    td a { color: #4f46e5; text-decoration: none; font-weight: 500; }
    td a:hover { text-decoration: underline; }
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
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class MerchantsComponent implements OnInit {
  merchants: any[] = [];
  loading = true;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getMerchants().subscribe({
      next: (res) => {
        this.merchants = res.data;
        this.loading = false;
      },
      error: () => this.loading = false
    });
  }
}
