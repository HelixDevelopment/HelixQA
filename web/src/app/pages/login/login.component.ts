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
          <p>{{ step === 'login' ? 'Sign in to your account' : 'Enter verification code' }}</p>
        </div>

        <form *ngIf="step === 'login'" (ngSubmit)="onSubmit()" #loginForm="ngForm">
          <div class="form-group">
            <label for="email">Email</label>
            <input
              id="email"
              type="email"
              [(ngModel)]="email"
              name="email"
              #emailField="ngModel"
              required
              email
              placeholder="you@example.com"
              [class.od-input-error]="formSubmitted && emailField.invalid"
            />
            <div class="field-error" *ngIf="formSubmitted && emailField.errors?.['required']">
              Email is required
            </div>
            <div class="field-error" *ngIf="formSubmitted && emailField.errors?.['email']">
              Enter a valid email address
            </div>
          </div>

          <div class="form-group">
            <label for="password">Password</label>
            <input
              id="password"
              type="password"
              [(ngModel)]="password"
              name="password"
              #passwordField="ngModel"
              required
              minlength="6"
              placeholder="Enter your password"
              [class.od-input-error]="formSubmitted && passwordField.invalid"
            />
            <div class="field-error" *ngIf="formSubmitted && passwordField.errors?.['required']">
              Password is required
            </div>
            <div class="field-error" *ngIf="formSubmitted && passwordField.errors?.['minlength']">
              Password must be at least 6 characters
            </div>
          </div>

          <div class="error-message" *ngIf="error">{{ error }}</div>

          <button type="submit" class="od-btn od-btn-primary" [disabled]="loading">
            {{ loading ? 'Signing in...' : 'Sign In' }}
          </button>
        </form>

        <form *ngIf="step === 'mfa'" (ngSubmit)="verifyMfa()" #mfaForm="ngForm">
          <p class="mfa-description">Enter the 6-digit code from your authenticator app.</p>
          <div class="form-group">
            <label for="code">Verification Code</label>
            <input
              id="code"
              type="text"
              [(ngModel)]="code"
              name="code"
              #codeField="ngModel"
              required
              pattern="[0-9]{6}"
              placeholder="000000"
              inputmode="numeric"
              autocomplete="one-time-code"
              [class.od-input-error]="formSubmitted && codeField.invalid"
            />
            <div class="field-error" *ngIf="formSubmitted && codeField.errors?.['required']">
              Verification code is required
            </div>
            <div class="field-error" *ngIf="formSubmitted && codeField.errors?.['pattern']">
              Enter a valid 6-digit code
            </div>
          </div>

          <div class="error-message" *ngIf="error">{{ error }}</div>

          <button type="submit" class="od-btn od-btn-primary" [disabled]="loading">
            {{ loading ? 'Verifying...' : 'Verify' }}
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
      background: var(--od-bg-secondary, #f5f5f5);
    }
    .login-card {
      background: var(--od-card-bg, white);
      border-radius: var(--od-radius, 8px);
      padding: var(--od-spacing-xl, 40px);
      width: 100%;
      max-width: 400px;
      box-shadow: var(--od-card-shadow, 0 2px 10px rgba(0,0,0,0.1));
    }
    .login-header {
      text-align: center;
      margin-bottom: var(--od-spacing-xl, 32px);
    }
    .login-header h1 {
      margin: 0 0 var(--od-spacing-xs, 8px);
      font-size: 24px;
      color: var(--od-text-primary, #1a1a1a);
    }
    .login-header p {
      margin: 0;
      color: var(--od-text-secondary, #666);
      font-size: 14px;
    }
    .mfa-description {
      color: var(--od-text-secondary, #666);
      font-size: 14px;
      margin-bottom: var(--od-spacing-lg, 20px);
      text-align: center;
    }
    .form-group {
      margin-bottom: var(--od-spacing-lg, 20px);
    }
    .form-group label {
      display: block;
      margin-bottom: var(--od-spacing-xs, 6px);
      font-size: 14px;
      font-weight: 500;
      color: var(--od-text-primary, #333);
    }
    .form-group input {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid var(--od-border, #ddd);
      border-radius: var(--od-radius-sm, 6px);
      font-size: 14px;
      box-sizing: border-box;
      background: var(--od-bg-primary, white);
      color: var(--od-text-primary, #333);
    }
    .form-group input:focus {
      outline: none;
      border-color: var(--od-accent, #4a90d9);
      box-shadow: 0 0 0 3px rgba(74,144,217,0.15);
    }
    .od-input-error {
      border-color: var(--od-danger, #dc3545) !important;
    }
    .field-error {
      color: var(--od-danger, #dc3545);
      font-size: 12px;
      margin-top: 4px;
    }
    .error-message {
      color: var(--od-danger, #dc3545);
      font-size: 14px;
      margin-bottom: var(--od-spacing-md, 16px);
      padding: 8px 12px;
      background: rgba(220, 53, 69, 0.08);
      border-radius: var(--od-radius-sm, 6px);
    }
    .od-btn {
      width: 100%;
      padding: 10px;
      border: none;
      border-radius: var(--od-radius-sm, 6px);
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
    }
    .od-btn-primary {
      background: var(--od-accent, #4a90d9);
      color: white;
    }
    .od-btn-primary:hover:not(:disabled) {
      filter: brightness(0.9);
    }
    .od-btn-primary:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
  `]
})
export class LoginComponent {
  step: 'login' | 'mfa' = 'login';
  email = '';
  password = '';
  code = '';
  mfaToken = '';
  loading = false;
  formSubmitted = false;
  error = '';

  constructor(private api: ApiService, private router: Router) {}

  onSubmit(): void {
    this.formSubmitted = true;
    if (!this.email || !this.password) return;
    this.login();
  }

  login(): void {
    this.loading = true;
    this.error = '';
    this.api.post<{ token?: string; mfa_required?: boolean; mfa_token?: string }>('/auth/login', {
      email: this.email,
      password: this.password
    }).subscribe({
      next: (res) => {
        if (res.mfa_required && res.mfa_token) {
          this.mfaToken = res.mfa_token;
          this.step = 'mfa';
          this.formSubmitted = false;
          this.loading = false;
        } else if (res.token) {
          localStorage.setItem('helix_token', res.token);
          this.router.navigate(['/dashboard']);
        }
      },
      error: (err) => {
        this.error = err.error?.error || 'Login failed';
        this.loading = false;
      }
    });
  }

  verifyMfa(): void {
    this.formSubmitted = true;
    if (!this.code) return;
    this.loading = true;
    this.error = '';
    this.api.post<{ token: string }>('/auth/mfa/verify', {
      code: this.code,
      token: this.mfaToken
    }).subscribe({
      next: (res) => {
        localStorage.setItem('helix_token', res.token);
        this.router.navigate(['/dashboard']);
      },
      error: (err) => {
        this.error = err.error?.error || 'Verification failed';
        this.loading = false;
      }
    });
  }
}
