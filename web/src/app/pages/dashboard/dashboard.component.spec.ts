import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { DashboardComponent } from './dashboard.component';

describe('DashboardComponent', () => {
  let component: DashboardComponent;
  let fixture: ComponentFixture<DashboardComponent>;
  let httpMock: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [DashboardComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(DashboardComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/analytics/summary');
    req.flush({});
    expect(component).toBeTruthy();
  });

  it('should have default summary values before data loads', () => {
    expect(component.summary.total_revenue).toBe(0);
    expect(component.summary.total_transactions).toBe(0);
    expect(component.summary.period).toBe('');
  });

  it('should fetch analytics summary on init', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/analytics/summary');
    expect(req.request.method).toBe('GET');
    req.flush({
      total_revenue: 5000,
      total_transactions: 100,
      successful_transactions: 90,
      failed_transactions: 10,
      period: 'July 2026'
    });

    expect(component.summary.total_revenue).toBe(5000);
    expect(component.summary.total_transactions).toBe(100);
    expect(component.summary.successful_transactions).toBe(90);
    expect(component.summary.failed_transactions).toBe(10);
    expect(component.summary.period).toBe('July 2026');
  });

  it('should merge partial analytics data with defaults', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/analytics/summary');
    req.flush({ total_revenue: 1000 });

    expect(component.summary.total_revenue).toBe(1000);
    expect(component.summary.total_transactions).toBe(0);
    expect(component.summary.failed_transactions).toBe(0);
  });

  it('should render dashboard heading', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/analytics/summary');
    req.flush({});
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Dashboard');
  });

  it('should render stat cards', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/analytics/summary').flush({
      total_revenue: 12345,
      total_transactions: 500,
      successful_transactions: 480,
      failed_transactions: 20,
      period: 'June 2026'
    });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const statCards = el.querySelectorAll('.stat-card');
    expect(statCards.length).toBe(4);
  });

  it('should render quick links section', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/analytics/summary').flush({});
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const links = el.querySelectorAll('.link-card');
    expect(links.length).toBe(4);
  });

  it('should render quick links with correct text', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/analytics/summary').flush({});
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const titles = Array.from(el.querySelectorAll('.link-title')).map(e => e.textContent?.trim());
    expect(titles).toContain('Merchants');
    expect(titles).toContain('Transactions');
    expect(titles).toContain('Customers');
    expect(titles).toContain('Subscriptions');
  });

  it('should handle analytics API error gracefully', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/analytics/summary');
    req.flush('error', { status: 500, statusText: 'Server Error' });

    expect(component.summary.total_revenue).toBe(0);
    expect(component.summary.period).toBe('');
  });
});
