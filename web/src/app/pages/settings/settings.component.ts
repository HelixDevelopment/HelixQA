import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="settings-page">
      <h1>Settings</h1>
      <div class="settings-card">
        <p class="placeholder">Settings page coming soon.</p>
      </div>
    </div>
  `,
  styles: [`
    .settings-page { padding: 24px; }
    h1 { margin: 0 0 24px; font-size: 24px; color: #1a1a1a; }
    .settings-card {
      background: white;
      border-radius: 8px;
      padding: 40px;
      box-shadow: 0 1px 3px rgba(0,0,0,0.1);
      text-align: center;
    }
    .placeholder { color: #999; font-size: 16px; }
  `]
})
export class SettingsComponent {}
