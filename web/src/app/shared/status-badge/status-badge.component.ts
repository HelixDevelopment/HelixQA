import { Component, Input } from '@angular/core';
import { NgClass } from '@angular/common';

@Component({
  selector: 'app-status-badge',
  standalone: true,
  imports: [NgClass],
  template: ` <span class="badge" [ngClass]="'badge-' + variant">{{ label }}</span> `,
  styles: [`
    .badge { display: inline-block; padding: 0.25em 0.65em; font-size: 0.75rem; font-weight: 600; border-radius: 999px; text-transform: uppercase; letter-spacing: 0.03em; }
    .badge-active, .badge-success, .badge-healthy, .badge-completed { background: #d4edda; color: #155724; }
    .badge-pending, .badge-warning { background: #fff3cd; color: #856404; }
    .badge-suspended, .badge-failed, .badge-declined, .badge-unhealthy { background: #f8d7da; color: #721c24; }
  `]
})
export class StatusBadgeComponent {
  @Input() label = '';
  @Input() variant: string = 'active';
}
