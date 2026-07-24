import { Component, Input, Output, EventEmitter } from '@angular/core';

@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  template: `
    <div class="overlay" (click)="cancel.emit()">
      <div class="dialog" (click)="$event.stopPropagation()">
        <h3>{{ title }}</h3>
        <p>{{ message }}</p>
        <div class="dialog-actions">
          <button class="btn" (click)="cancel.emit()">Cancel</button>
          <button class="btn btn-danger" (click)="confirm.emit()">{{ confirmLabel }}</button>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
    .dialog { background: var(--od-card-bg); border-radius: var(--od-radius); padding: var(--od-spacing-xl); max-width: 400px; width: 90%; }
    .dialog h3 { margin-bottom: var(--od-spacing-sm); }
    .dialog p { color: var(--od-text-secondary); margin-bottom: var(--od-spacing-lg); }
    .dialog-actions { display: flex; justify-content: flex-end; gap: var(--od-spacing-sm); }
    .btn { padding: 0.5rem 1rem; border-radius: var(--od-radius-sm); border: 1px solid var(--od-border); cursor: pointer; }
    .btn-danger { background: var(--od-danger); color: white; border-color: var(--od-danger); }
  `]
})
export class ConfirmDialogComponent {
  @Input() title = 'Confirm';
  @Input() message = 'Are you sure?';
  @Input() confirmLabel = 'Delete';
  @Output() confirm = new EventEmitter<void>();
  @Output() cancel = new EventEmitter<void>();
}
