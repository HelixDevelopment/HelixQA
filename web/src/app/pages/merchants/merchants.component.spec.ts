import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { MerchantsComponent } from './merchants.component';

describe('MerchantsComponent', () => {
  let component: MerchantsComponent;
  let fixture: ComponentFixture<MerchantsComponent>;
  let httpMock: HttpTestingController;

  const mockMerchants = [
    { id: 'm1', name: 'Acme Corp', email: 'acme@test.com', country: 'US', currency: 'USD', status: 'active', created_at: '2026-01-01T00:00:00Z' },
    { id: 'm2', name: 'Beta LLC', trade_name: 'Beta Store', email: 'beta@test.com', country: 'GB', currency: 'GBP', status: 'pending', created_at: '2026-02-01T00:00:00Z' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MerchantsComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(MerchantsComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load merchants on init', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne(r => r.url === '/api/merchants');
    req.flush({ data: mockMerchants, total: 2, page: 1, per_page: 20 });

    expect(component.merchants.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false on error', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne(r => r.url === '/api/merchants');
    req.flush('error', { status: 500, statusText: 'Server Error' });

    expect(component.loading).toBeFalse();
    expect(component.merchants.length).toBe(0);
  });

  it('should render table with merchant data', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: mockMerchants, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display merchant name or trade_name', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: mockMerchants, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const cells = el.querySelectorAll('tbody td:first-child');
    expect(cells[0]?.textContent).toContain('Acme Corp');
    expect(cells[1]?.textContent).toContain('Beta Store');
  });

  it('should display merchant status badge', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: mockMerchants, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges.length).toBe(2);
    expect(badges[0]?.textContent?.trim()).toBe('active');
    expect(badges[1]?.textContent?.trim()).toBe('pending');
  });

  it('should show empty state when no merchants', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No merchants found');
  });

  it('should have page heading', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Merchants');
  });
});
