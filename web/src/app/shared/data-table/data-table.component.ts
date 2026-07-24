import { Component, Input, Output, EventEmitter } from '@angular/core';
import { NgFor, NgIf } from '@angular/common';

@Component({
  selector: 'app-data-table',
  standalone: true,
  imports: [NgFor, NgIf],
  template: `
    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th *ngFor="let col of columns">{{ col.label }}</th>
            <th *ngIf="actions">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr *ngFor="let row of data">
            <td *ngFor="let col of columns">{{ row[col.key] }}</td>
            <td *ngIf="actions" class="actions-cell">
              <button *ngFor="let action of actions" class="btn btn-sm" 
                [class.btn-primary]="action.primary"
                (click)="action.onClick(row)">{{ action.label }}</button>
            </td>
          </tr>
          <tr *ngIf="data.length === 0">
            <td [attr.colspan]="columns.length + (actions ? 1 : 0)" class="empty-state">
              No data available
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  `,
  styles: [`
    .table-container { overflow-x: auto; }
    .data-table { width: 100%; border-collapse: collapse; }
    .data-table th, .data-table td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid var(--od-border); }
    .data-table th { font-weight: 600; color: var(--od-text-secondary); font-size: 0.875rem; text-transform: uppercase; letter-spacing: 0.05em; }
    .data-table tbody tr:hover { background: var(--od-bg-secondary); }
    .actions-cell { white-space: nowrap; }
    .btn { padding: 0.375rem 0.75rem; border: 1px solid var(--od-border); border-radius: var(--od-radius-sm); background: transparent; cursor: pointer; font-size: 0.875rem; }
    .btn:hover { background: var(--od-bg-secondary); }
    .btn-primary { background: var(--od-accent); color: white; border-color: var(--od-accent); }
    .empty-state { text-align: center; padding: 2rem; color: var(--od-text-muted); }
  `]
})
export class DataTableComponent {
  @Input() columns: { key: string; label: string }[] = [];
  @Input() data: any[] = [];
  @Input() actions?: { label: string; primary?: boolean; onClick: (row: any) => void }[];
}
