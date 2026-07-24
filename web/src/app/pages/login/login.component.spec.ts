import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { LoginComponent } from './login.component';

describe('LoginComponent', () => {
  let component: LoginComponent;
  let fixture: ComponentFixture<LoginComponent>;
  let httpMock: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LoginComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(LoginComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should have empty initial state', () => {
    expect(component.email).toBe('');
    expect(component.password).toBe('');
    expect(component.loading).toBeFalse();
    expect(component.error).toBe('');
  });

  it('should render email and password inputs', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('#email')).toBeTruthy();
    expect(el.querySelector('#password')).toBeTruthy();
  });

  it('should render sign in button', () => {
    const el: HTMLElement = fixture.nativeElement;
    const button = el.querySelector('button[type="submit"]');
    expect(button).toBeTruthy();
    expect(button?.textContent?.trim()).toBe('Sign In');
  });

  it('should show header text', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.login-header h1')?.textContent).toContain('Helix Seller');
  });

  it('should set loading to true and call API on submit', () => {
    component.email = 'test@example.com';
    component.password = 'password123';
    component.onSubmit();

    expect(component.loading).toBeTrue();
    expect(component.error).toBe('');

    const req = httpMock.expectOne('/api/auth/login');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual({ email: 'test@example.com', password: 'password123' });
    req.flush({ token: 'abc' });
  });

  it('should navigate to /dashboard on successful login', fakeAsync(() => {
    const routerSpy = spyOn(TestBed.inject(Router), 'navigate');

    component.email = 'test@example.com';
    component.password = 'password123';
    component.onSubmit();

    const req = httpMock.expectOne('/api/auth/login');
    req.flush({ token: 'abc' });
    tick();

    expect(routerSpy).toHaveBeenCalledWith(['/dashboard']);
  }));

  it('should set error message on login failure', () => {
    component.email = 'bad@example.com';
    component.password = 'wrong';
    component.onSubmit();

    const req = httpMock.expectOne('/api/auth/login');
    req.flush({ error: 'Invalid credentials' }, { status: 401, statusText: 'Unauthorized' });

    expect(component.error).toBe('Invalid credentials');
    expect(component.loading).toBeFalse();
  });

  it('should set default error message when response has no error field', () => {
    component.email = 'a@b.com';
    component.password = 'x';
    component.onSubmit();

    const req = httpMock.expectOne('/api/auth/login');
    req.flush({}, { status: 500, statusText: 'Server Error' });

    expect(component.error).toBe('Login failed');
  });

  it('should display error message in template', () => {
    component.error = 'Something went wrong';
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const errorEl = el.querySelector('.error-message');
    expect(errorEl?.textContent).toContain('Something went wrong');
  });

  it('should hide error message when error is empty', () => {
    component.error = '';
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const errorEl = el.querySelector('.error-message');
    expect(errorEl).toBeNull();
  });

  it('should disable button when loading', () => {
    component.loading = true;
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const button = el.querySelector('button[type="submit"]') as HTMLButtonElement;
    expect(button.disabled).toBeTrue();
  });

  it('should show loading text on button when loading', () => {
    component.loading = true;
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const button = el.querySelector('button[type="submit"]');
    expect(button?.textContent?.trim()).toBe('Signing in...');
  });
});
