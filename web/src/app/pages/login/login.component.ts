import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { ApiService } from '../../core/api.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div class="login-container">
      <div class="login-card">
        <div class="login-header">
          <h1>Helix Seller</h1>
          <p>Sign in to your account</p>
        </div>
        <form (ngSubmit)="onSubmit()">
          <div class="form-group">
            <label for="email">Email</label>
            <input
              id="email"
              type="email"
              [(ngModel)]="email"
              name="email"
              required
              placeholder="you@example.com"
            />
          </div>
          <div class="form-group">
            <label for="password">Password</label>
            <input
              id="password"
              type="password"
              [(ngModel)]="password"
              name="password"
              required
              placeholder="Enter your password"
            />
          </div>
          <div class="error-message" *ngIf="error">{{ error }}</div>
          <button type="submit" [disabled]="loading">
            {{ loading ? 'Signing in...' : 'Sign In' }}
          </button>
        </form>
      </div>
    </div>
  `,
  styles: [`
    .login-container {
      display: flex;
      justify-content: center;
      align-items: center;
      min-height: 100vh;
      background: #f5f5f5;
    }
    .login-card {
      background: white;
      border-radius: 8px;
      padding: 40px;
      width: 100%;
      max-width: 400px;
      box-shadow: 0 2px 10px rgba(0,0,0,0.1);
    }
    .login-header {
      text-align: center;
      margin-bottom: 32px;
    }
    .login-header h1 {
      margin: 0 0 8px;
      font-size: 24px;
      color: #1a1a1a;
    }
    .login-header p {
      margin: 0;
      color: #666;
      font-size: 14px;
    }
    .form-group {
      margin-bottom: 20px;
    }
    .form-group label {
      display: block;
      margin-bottom: 6px;
      font-size: 14px;
      font-weight: 500;
      color: #333;
    }
    .form-group input {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid #ddd;
      border-radius: 6px;
      font-size: 14px;
      box-sizing: border-box;
    }
    .form-group input:focus {
      outline: none;
      border-color: #4f46e5;
      box-shadow: 0 0 0 3px rgba(79,70,229,0.1);
    }
    .error-message {
      color: #dc2626;
      font-size: 14px;
      margin-bottom: 16px;
      padding: 8px 12px;
      background: #fef2f2;
      border-radius: 6px;
    }
    button {
      width: 100%;
      padding: 10px;
      background: #4f46e5;
      color: white;
      border: none;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
    }
    button:hover:not(:disabled) {
      background: #4338ca;
    }
    button:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
  `]
})
export class LoginComponent {
  email = '';
  password = '';
  loading = false;
  error = '';

  constructor(private api: ApiService, private router: Router) {}

  onSubmit(): void {
    this.loading = true;
    this.error = '';
    this.api.login({ email: this.email, password: this.password }).subscribe({
      next: () => {
        this.router.navigate(['/dashboard']);
      },
      error: (err) => {
        this.error = err.error?.error || 'Login failed';
        this.loading = false;
      }
    });
  }
}
