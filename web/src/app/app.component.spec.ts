import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { AppComponent } from './app.component';

describe('AppComponent', () => {
  let component: AppComponent;
  let fixture: ComponentFixture<AppComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AppComponent],
      providers: [provideRouter([])]
    }).compileComponents();

    fixture = TestBed.createComponent(AppComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should render the brand title', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.sidebar-brand h1')?.textContent).toContain('Helix Seller');
  });

  it('should render navigation links', () => {
    const el: HTMLElement = fixture.nativeElement;
    const links = el.querySelectorAll('.sidebar-nav a');
    expect(links.length).toBe(6);
  });

  it('should have link to /dashboard', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/dashboard"]');
    expect(link).toBeTruthy();
    expect(link?.textContent?.trim()).toBe('Dashboard');
  });

  it('should have link to /merchants', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/merchants"]');
    expect(link).toBeTruthy();
  });

  it('should have link to /transactions', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/transactions"]');
    expect(link).toBeTruthy();
  });

  it('should have link to /customers', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/customers"]');
    expect(link).toBeTruthy();
  });

  it('should have link to /subscriptions', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/subscriptions"]');
    expect(link).toBeTruthy();
  });

  it('should have link to /settings', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a[href="/settings"]');
    expect(link).toBeTruthy();
  });

  it('should contain a router-outlet', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('router-outlet')).toBeTruthy();
  });
});
