import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { TransactionsComponent } from './transactions.component';

describe('TransactionsComponent', () => {
  let component: TransactionsComponent;
  let fixture: ComponentFixture<TransactionsComponent>;
  let httpMock: HttpTestingController;

  const mockTransactions = [
    { id: 'tx1', merchant_id: 'm1', amount: 5000, currency: 'USD', type: 'payment', provider: 'stripe', status: 'succeeded', created_at: '2026-07-01T10:00:00Z' },
    { id: 'tx2', merchant_id: 'm2', amount: 1200, currency: 'EUR', type: 'refund', provider: 'paypal', status: 'pending', created_at: '2026-07-02T12:00:00Z' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TransactionsComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(TransactionsComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load transactions on init', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });

    expect(component.transactions.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false on error', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush('error', { status: 500, statusText: 'Error' });

    expect(component.loading).toBeFalse();
    expect(component.transactions.length).toBe(0);
  });

  it('should render transaction rows', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display transaction ID', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const idCells = el.querySelectorAll('.id-cell');
    expect(idCells[0]?.textContent?.trim()).toBe('tx1');
  });

  it('should display formatted amount in dollars', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const amountCells = el.querySelectorAll('.amount-cell');
    expect(amountCells[0]?.textContent).toContain('50.00');
    expect(amountCells[1]?.textContent).toContain('12.00');
  });

  it('should display status badge', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges[0]?.textContent?.trim()).toBe('succeeded');
    expect(badges[1]?.textContent?.trim()).toBe('pending');
  });

  it('should link to merchant detail', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: mockTransactions, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const merchantLinks = el.querySelectorAll('a[href="/merchants/m1"]');
    expect(merchantLinks.length).toBe(1);
  });

  it('should show empty state when no transactions', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No transactions found');
  });

  it('should have page heading', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/transactions').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Transactions');
  });
});
