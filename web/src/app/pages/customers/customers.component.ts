import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-customers',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="page">
      <div class="page-header">
        <h1>Customers</h1>
      </div>

      <div class="filters">
        <input
          type="text"
          placeholder="Search by name..."
          [(ngModel)]="searchTerm"
          (input)="onSearchChange()"
        />
      </div>

      <div class="loading-overlay" *ngIf="loading">
        <div class="spinner"></div>
      </div>

      <div class="error-state" *ngIf="error">
        <p>Failed to load customers.</p>
        <button class="btn" (click)="loadCustomers()">Retry</button>
      </div>

      <ng-container *ngIf="!loading && !error">
        <div class="table-container">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Phone</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <tr *ngFor="let customer of filteredCustomers">
                <td>{{ customer.name }}</td>
                <td>{{ customer.email }}</td>
                <td>{{ customer.phone || '-' }}</td>
                <td>{{ customer.created_at | date:'mediumDate' }}</td>
              </tr>
            </tbody>
          </table>

          <div class="empty-state" *ngIf="filteredCustomers.length === 0">No customers found.</div>
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
    input {
      padding: 8px 12px;
      border: 1px solid var(--od-border);
      border-radius: var(--od-radius-sm);
      background: var(--od-bg-primary);
      color: var(--od-text-primary);
      font-size: 14px;
      font-family: var(--od-font-sans);
      min-width: 240px;
    }
    input::placeholder { color: var(--od-text-muted); }

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
      overflow: hidden;
    }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 12px 16px; text-align: left; font-size: 14px; color: var(--od-text-primary); }
    th { background: var(--od-bg-secondary); color: var(--od-text-secondary); font-weight: 500; border-bottom: 1px solid var(--od-border); }
    td { border-bottom: 1px solid var(--od-border); }
    tr:last-child td { border-bottom: none; }

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
export class CustomersComponent implements OnInit {
  customers: any[] = [];
  filteredCustomers: any[] = [];
  loading = true;
  error = false;
  page = 1;
  perPage = 20;
  total = 0;
  totalPages = 0;
  searchTerm = '';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.loadCustomers();
  }

  loadCustomers(): void {
    this.loading = true;
    this.error = false;
    this.api.getCustomers(this.page, this.perPage).subscribe({
      next: (res) => {
        this.customers = res.data;
        this.total = res.total;
        this.totalPages = Math.ceil(res.total / res.per_page);
        this.filteredCustomers = this.searchTerm
          ? this.customers.filter(c => c.name.toLowerCase().includes(this.searchTerm.toLowerCase()))
          : [...this.customers];
        this.loading = false;
      },
      error: () => {
        this.loading = false;
        this.error = true;
      }
    });
  }

  onSearchChange(): void {
    this.page = 1;
    this.loadCustomers();
  }

  goToPage(p: number): void {
    this.page = p;
    this.loadCustomers();
  }
}
