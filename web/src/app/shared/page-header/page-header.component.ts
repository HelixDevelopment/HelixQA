import { Component, Input } from '@angular/core';
import { NgIf } from '@angular/common';

@Component({
  selector: 'app-page-header',
  standalone: true,
  imports: [NgIf],
  template: `
    <div class="page-header">
      <h1 class="page-title">{{ title }}</h1>
      <p *ngIf="subtitle" class="page-subtitle">{{ subtitle }}</p>
      <ng-content></ng-content>
    </div>
  `,
  styles: [`
    .page-header { margin-bottom: var(--od-spacing-xl); }
    .page-title { font-size: 1.75rem; font-weight: 700; color: var(--od-text-primary); margin: 0; }
    .page-subtitle { margin-top: 0.25rem; color: var(--od-text-secondary); font-size: 0.95rem; }
  `]
})
export class PageHeaderComponent {
  @Input() title = '';
  @Input() subtitle = '';
}
