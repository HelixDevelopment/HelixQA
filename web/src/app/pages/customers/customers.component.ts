import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-customers',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="customers-page">
      <div class="page-header">
        <h1>Customers</h1>
      </div>
      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Phone</th>
              <th>External ID</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr *ngFor="let customer of customers">
              <td>{{ customer.name }}</td>
              <td>{{ customer.email }}</td>
              <td>{{ customer.phone || '-' }}</td>
              <td class="id-cell">{{ customer.external_id || '-' }}</td>
              <td>{{ customer.created_at | date:'mediumDate' }}</td>
            </tr>
          </tbody>
        </table>
        <div class="empty-state" *ngIf="customers.length === 0 && !loading">No customers found.</div>
      </div>
    </div>
  `,
  styles: [`
    .customers-page { padding: 24px; }
    .page-header { margin-bottom: 20px; }
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
    .id-cell { font-family: monospace; font-size: 12px; color: #666; }
    .empty-state { padding: 40px; text-align: center; color: #999; }
  `]
})
export class CustomersComponent implements OnInit {
  customers: any[] = [];
  loading = true;

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getCustomers().subscribe({
      next: (res) => {
        this.customers = res.data;
        this.loading = false;
      },
      error: () => this.loading = false
    });
  }
}
