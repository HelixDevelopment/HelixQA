import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { CustomersComponent } from './customers.component';

describe('CustomersComponent', () => {
  let component: CustomersComponent;
  let fixture: ComponentFixture<CustomersComponent>;
  let httpMock: HttpTestingController;

  const mockCustomers = [
    { id: 'c1', name: 'John Doe', email: 'john@test.com', phone: '+1234567890', external_id: 'ext-001', created_at: '2026-01-10T00:00:00Z' },
    { id: 'c2', name: 'Jane Smith', email: 'jane@test.com', created_at: '2026-03-15T00:00:00Z' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CustomersComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(CustomersComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load customers on init', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne(r => r.url === '/api/customers');
    req.flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });

    expect(component.customers.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false and error true on error', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne(r => r.url === '/api/customers');
    req.flush('error', { status: 500, statusText: 'Error' });

    expect(component.loading).toBeFalse();
    expect(component.error).toBeTrue();
  });

  it('should show error state with retry button', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush('error', { status: 500, statusText: 'Error' });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.error-state')).toBeTruthy();
    expect(el.textContent).toContain('Retry');
  });

  it('should render customer rows', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display customer name and email', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('John Doe');
    expect(el.textContent).toContain('john@test.com');
  });

  it('should display phone or dash for missing phone', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    const firstRowCells = rows[0].querySelectorAll('td');
    expect(firstRowCells[2]?.textContent?.trim()).toBe('+1234567890');
    const secondRowCells = rows[1].querySelectorAll('td');
    expect(secondRowCells[2]?.textContent?.trim()).toBe('-');
  });

  it('should show empty state when no customers', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No customers found');
  });

  it('should have page heading', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Customers');
  });

  it('should filter customers by search term', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    component.searchTerm = 'Jane';
    component.onSearchChange();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    expect(component.filteredCustomers.length).toBe(1);
    expect(component.filteredCustomers[0].name).toBe('Jane Smith');
  });

  it('should show pagination when multiple pages', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: mockCustomers, total: 40, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.pagination')).toBeTruthy();
  });

  it('should show loading spinner during fetch', () => {
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.loading-overlay')).toBeTruthy();

    httpMock.expectOne(r => r.url === '/api/customers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();
    expect(el.querySelector('.loading-overlay')).toBeFalsy();
  });
});
