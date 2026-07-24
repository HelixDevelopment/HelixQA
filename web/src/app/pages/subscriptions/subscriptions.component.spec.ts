import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { SubscriptionsComponent } from './subscriptions.component';

describe('SubscriptionsComponent', () => {
  let component: SubscriptionsComponent;
  let fixture: ComponentFixture<SubscriptionsComponent>;
  let httpMock: HttpTestingController;

  const mockSubscriptions = [
    {
      id: 's1', merchant_id: 'm1', customer_id: 'c1', plan_id: 'plan-a', amount: 2999, interval: 'month',
      interval_count: 1, status: 'active', current_period_start: '2026-07-01T00:00:00Z',
      current_period_end: '2026-08-01T00:00:00Z', created_at: '2026-01-01T00:00:00Z'
    },
    {
      id: 's2', merchant_id: 'm1', customer_id: 'c2', plan_id: 'plan-b', amount: 9900, interval: 'year',
      interval_count: 1, status: 'past_due', current_period_start: '2026-01-01T00:00:00Z',
      current_period_end: '2027-01-01T00:00:00Z', created_at: '2026-01-15T00:00:00Z'
    },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [SubscriptionsComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(SubscriptionsComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load subscriptions on init', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });

    expect(component.subscriptions.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false on error', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush('error', { status: 500, statusText: 'Error' });

    expect(component.loading).toBeFalse();
    expect(component.subscriptions.length).toBe(0);
  });

  it('should render subscription rows', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display subscription ID', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const idCells = el.querySelectorAll('.id-cell');
    expect(idCells[0]?.textContent?.trim()).toBe('s1');
  });

  it('should display formatted amount', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const amountCells = el.querySelectorAll('.amount-cell');
    expect(amountCells[0]?.textContent).toContain('29.99');
    expect(amountCells[1]?.textContent).toContain('99.00');
  });

  it('should display status badge', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges[0]?.textContent?.trim()).toBe('active');
    expect(badges[1]?.textContent?.trim()).toBe('past_due');
  });

  it('should display interval text', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: mockSubscriptions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('month');
    expect(el.textContent).toContain('year');
  });

  it('should show empty state when no subscriptions', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No subscriptions found');
  });

  it('should have page heading', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/subscriptions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Subscriptions');
  });
});
