import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { ProvidersComponent } from './providers.component';

describe('ProvidersComponent', () => {
  let component: ProvidersComponent;
  let fixture: ComponentFixture<ProvidersComponent>;
  let httpMock: HttpTestingController;

  const mockProviders = [
    { id: 'p1', merchant_id: 'default', provider: 'stripe', status: 'active', config: {}, created_at: '2026-01-01T00:00:00Z' },
    { id: 'p2', merchant_id: 'default', provider: 'paypal', status: 'inactive', config: {}, created_at: '2026-03-01T00:00:00Z' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ProvidersComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(ProvidersComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load providers on init', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });

    expect(component.providers.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false on error', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush('error', { status: 500, statusText: 'Error' });

    expect(component.loading).toBeFalse();
  });

  it('should render provider rows', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display provider names', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('Stripe');
    expect(el.textContent).toContain('Paypal');
  });

  it('should display status badge', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges[0]?.textContent?.trim()).toBe('active');
    expect(badges[1]?.textContent?.trim()).toBe('inactive');
  });

  it('should toggle form visibility', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });

    expect(component.showForm).toBeFalse();

    component.showForm = true;
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.form-card')).toBeTruthy();

    component.showForm = false;
    fixture.detectChanges();
    expect(el.querySelector('.form-card')).toBeNull();
  });

  it('should show empty state when no providers', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No providers configured');
  });

  it('should create provider on submit', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newProvider = { provider: 'stripe' };
    component.apiKey = 'sk_test_123';
    component.webhookSecret = 'whsec_abc';
    component.onSubmit();

    expect(component.submitting).toBeTrue();
    const req = httpMock.expectOne('/api/merchants/default/providers');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(jasmine.objectContaining({
      provider: 'stripe',
      config: { api_key: 'sk_test_123', webhook_secret: 'whsec_abc' }
    }));
    req.flush({ id: 'p3', ...component.newProvider, status: 'active', config: {}, created_at: '' });
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    tick();

    expect(component.showForm).toBeFalse();
    expect(component.submitting).toBeFalse();
    expect(component.apiKey).toBe('');
    expect(component.webhookSecret).toBe('');
  }));

  it('should reset submitting on create error', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newProvider = { provider: 'stripe' };
    component.apiKey = 'sk_test_123';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/providers');
    req.flush('error', { status: 400, statusText: 'Bad Request' });
    tick();

    expect(component.submitting).toBeFalse();
  }));

  it('should delete provider after confirm', fakeAsync(() => {
    spyOn(window, 'confirm').and.returnValue(true);
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });

    component.onDelete('p1');

    const req = httpMock.expectOne('/api/merchants/default/providers/p1');
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    tick();

    expect(window.confirm).toHaveBeenCalled();
  }));

  it('should not delete provider when confirm is cancelled', () => {
    spyOn(window, 'confirm').and.returnValue(false);
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: mockProviders, total: 2, page: 1, per_page: 20 });

    component.onDelete('p1');

    const unmatched = httpMock.match(r => r.url === '/api/merchants/default/providers/p1');
    expect(unmatched.length).toBe(0);
  });

  it('should not include webhook_secret when empty', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newProvider = { provider: 'paypal' };
    component.apiKey = 'key123';
    component.webhookSecret = '';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/providers');
    const body = req.request.body as any;
    expect(body.config).toEqual({ api_key: 'key123' });
    expect(body.config['webhook_secret']).toBeUndefined();
    req.flush({ id: 'p4', provider: 'paypal', status: 'active', config: {}, merchant_id: 'default', created_at: '' });
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/providers').flush({ data: [], total: 0, page: 1, per_page: 20 });
    tick();
  }));
});
