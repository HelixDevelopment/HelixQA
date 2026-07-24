import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { ApiService } from './api.service';

describe('ApiService', () => {
  let service: ApiService;
  let httpMock: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
      ]
    });
    service = TestBed.inject(ApiService);
    httpMock = TestBed.inject(HttpTestingController);
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  describe('login', () => {
    it('should POST to /api/auth/login with credentials', () => {
      const credentials = { email: 'test@example.com', password: 'pass' };
      service.login(credentials).subscribe(res => {
        expect(res).toEqual({ token: 'abc' });
      });
      const req = httpMock.expectOne('/api/auth/login');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(credentials);
      req.flush({ token: 'abc' });
    });
  });

  describe('logout', () => {
    it('should POST to /api/auth/logout', () => {
      service.logout().subscribe();
      const req = httpMock.expectOne('/api/auth/logout');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual({});
      req.flush(null);
    });
  });

  describe('getDashboard', () => {
    it('should GET /api/dashboard', () => {
      service.getDashboard().subscribe(data => {
        expect(data).toEqual({ stats: 'ok' });
      });
      const req = httpMock.expectOne('/api/dashboard');
      expect(req.request.method).toBe('GET');
      req.flush({ stats: 'ok' });
    });
  });

  describe('getMerchants', () => {
    it('should GET /api/merchants with default pagination', () => {
      service.getMerchants().subscribe(res => {
        expect(res.total).toBe(1);
      });
      const req = httpMock.expectOne(r => r.url === '/api/merchants');
      expect(req.request.params.get('page')).toBe('1');
      expect(req.request.params.get('per_page')).toBe('20');
      req.flush({ data: [], total: 1, page: 1, per_page: 20 });
    });

    it('should GET /api/merchants with custom pagination', () => {
      service.getMerchants(3, 50).subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/merchants');
      expect(req.request.params.get('page')).toBe('3');
      expect(req.request.params.get('per_page')).toBe('50');
      req.flush({ data: [], total: 0, page: 3, per_page: 50 });
    });
  });

  describe('getMerchant', () => {
    it('should GET /api/merchants/:id', () => {
      service.getMerchant('m1').subscribe(m => {
        expect(m.name).toBe('Test Merchant');
      });
      const req = httpMock.expectOne('/api/merchants/m1');
      expect(req.request.method).toBe('GET');
      req.flush({ id: 'm1', name: 'Test Merchant', email: 'a@b.com', status: 'active', created_at: '' });
    });
  });

  describe('createMerchant', () => {
    it('should POST to /api/merchants', () => {
      const body = { name: 'New', email: 'n@b.com' };
      service.createMerchant(body).subscribe(m => {
        expect(m.id).toBe('m2');
      });
      const req = httpMock.expectOne('/api/merchants');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(body);
      req.flush({ id: 'm2', name: 'New', email: 'n@b.com', status: 'active', created_at: '' });
    });
  });

  describe('updateMerchant', () => {
    it('should PUT to /api/merchants/:id', () => {
      const body = { name: 'Updated' };
      service.updateMerchant('m1', body).subscribe(m => {
        expect(m.name).toBe('Updated');
      });
      const req = httpMock.expectOne('/api/merchants/m1');
      expect(req.request.method).toBe('PUT');
      expect(req.request.body).toEqual(body);
      req.flush({ id: 'm1', name: 'Updated', email: '', status: 'active', created_at: '' });
    });
  });

  describe('deleteMerchant', () => {
    it('should DELETE /api/merchants/:id', () => {
      service.deleteMerchant('m1').subscribe();
      const req = httpMock.expectOne('/api/merchants/m1');
      expect(req.request.method).toBe('DELETE');
      req.flush(null);
    });
  });

  describe('getTransactions', () => {
    it('should GET /api/transactions with default params', () => {
      service.getTransactions().subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/transactions');
      expect(req.request.params.get('page')).toBe('1');
      expect(req.request.params.has('merchant_id')).toBeFalse();
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });

    it('should include merchant_id when provided', () => {
      service.getTransactions(1, 20, 'm1').subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/transactions');
      expect(req.request.params.get('merchant_id')).toBe('m1');
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });
  });

  describe('getTransaction', () => {
    it('should GET /api/transactions/:id', () => {
      service.getTransaction('t1').subscribe(tx => {
        expect(tx.id).toBe('t1');
      });
      const req = httpMock.expectOne('/api/transactions/t1');
      expect(req.request.method).toBe('GET');
      req.flush({ id: 't1', merchant_id: 'm1', amount: 100, currency: 'USD', type: 'payment', provider: 'stripe', status: 'succeeded', created_at: '' });
    });
  });

  describe('getCustomers', () => {
    it('should GET /api/customers', () => {
      service.getCustomers().subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/customers');
      expect(req.request.method).toBe('GET');
      expect(req.request.params.get('page')).toBe('1');
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });
  });

  describe('getCustomer', () => {
    it('should GET /api/customers/:id', () => {
      service.getCustomer('c1').subscribe(c => {
        expect(c.id).toBe('c1');
      });
      const req = httpMock.expectOne('/api/customers/c1');
      req.flush({ id: 'c1', email: 'a@b.com', name: 'Test', created_at: '' });
    });
  });

  describe('getSubscriptions', () => {
    it('should GET /api/subscriptions', () => {
      service.getSubscriptions().subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/subscriptions');
      expect(req.request.params.get('page')).toBe('1');
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });
  });

  describe('getSubscription', () => {
    it('should GET /api/subscriptions/:id', () => {
      service.getSubscription('s1').subscribe(s => {
        expect(s.id).toBe('s1');
      });
      const req = httpMock.expectOne('/api/subscriptions/s1');
      req.flush({ id: 's1', merchant_id: 'm1', customer_id: 'c1', plan_id: 'p1', amount: 1000, interval: 'month', interval_count: 1, status: 'active', current_period_start: '', current_period_end: '', created_at: '' });
    });
  });

  describe('getSettings', () => {
    it('should GET /api/settings', () => {
      service.getSettings().subscribe();
      const req = httpMock.expectOne('/api/settings');
      expect(req.request.method).toBe('GET');
      req.flush({});
    });
  });

  describe('updateSettings', () => {
    it('should PUT /api/settings', () => {
      const settings = { key: 'value' };
      service.updateSettings(settings).subscribe();
      const req = httpMock.expectOne('/api/settings');
      expect(req.request.method).toBe('PUT');
      expect(req.request.body).toEqual(settings);
      req.flush({});
    });
  });

  describe('getAnalyticsSummary', () => {
    it('should GET /api/analytics/summary', () => {
      service.getAnalyticsSummary().subscribe();
      const req = httpMock.expectOne('/api/analytics/summary');
      expect(req.request.method).toBe('GET');
      req.flush({});
    });
  });

  describe('getProviders', () => {
    it('should GET /api/merchants/:merchantId/providers', () => {
      service.getProviders('m1').subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/merchants/m1/providers');
      expect(req.request.method).toBe('GET');
      expect(req.request.params.get('page')).toBe('1');
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });
  });

  describe('getProvider', () => {
    it('should GET /api/merchants/:merchantId/providers/:id', () => {
      service.getProvider('m1', 'p1').subscribe(p => {
        expect(p.id).toBe('p1');
      });
      const req = httpMock.expectOne('/api/merchants/m1/providers/p1');
      req.flush({ id: 'p1', merchant_id: 'm1', provider: 'stripe', status: 'active', config: {}, created_at: '' });
    });
  });

  describe('createProvider', () => {
    it('should POST /api/merchants/:merchantId/providers', () => {
      const body = { provider: 'stripe' };
      service.createProvider('m1', body).subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/providers');
      expect(req.request.method).toBe('POST');
      expect(req.request.body).toEqual(body);
      req.flush({ id: 'p1', merchant_id: 'm1', provider: 'stripe', status: 'active', config: {}, created_at: '' });
    });
  });

  describe('updateProvider', () => {
    it('should PUT /api/merchants/:merchantId/providers/:id', () => {
      const body = { status: 'inactive' };
      service.updateProvider('m1', 'p1', body).subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/providers/p1');
      expect(req.request.method).toBe('PUT');
      req.flush({ id: 'p1', merchant_id: 'm1', provider: 'stripe', status: 'inactive', config: {}, created_at: '' });
    });
  });

  describe('deleteProvider', () => {
    it('should DELETE /api/merchants/:merchantId/providers/:id', () => {
      service.deleteProvider('m1', 'p1').subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/providers/p1');
      expect(req.request.method).toBe('DELETE');
      req.flush(null);
    });
  });

  describe('getWebhooks', () => {
    it('should GET /api/merchants/:merchantId/webhooks', () => {
      service.getWebhooks('m1').subscribe();
      const req = httpMock.expectOne(r => r.url === '/api/merchants/m1/webhooks');
      expect(req.request.method).toBe('GET');
      req.flush({ data: [], total: 0, page: 1, per_page: 20 });
    });
  });

  describe('getWebhook', () => {
    it('should GET /api/merchants/:merchantId/webhooks/:id', () => {
      service.getWebhook('m1', 'w1').subscribe(w => {
        expect(w.id).toBe('w1');
      });
      const req = httpMock.expectOne('/api/merchants/m1/webhooks/w1');
      req.flush({ id: 'w1', merchant_id: 'm1', url: 'https://hook.test', events: ['a'], status: 'active', secret: '', created_at: '' });
    });
  });

  describe('createWebhook', () => {
    it('should POST /api/merchants/:merchantId/webhooks', () => {
      const body = { url: 'https://hook.test', events: ['a'] };
      service.createWebhook('m1', body).subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/webhooks');
      expect(req.request.method).toBe('POST');
      req.flush({ id: 'w1', merchant_id: 'm1', url: 'https://hook.test', events: ['a'], status: 'active', secret: '', created_at: '' });
    });
  });

  describe('updateWebhook', () => {
    it('should PUT /api/merchants/:merchantId/webhooks/:id', () => {
      const body = { url: 'https://updated.test' };
      service.updateWebhook('m1', 'w1', body).subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/webhooks/w1');
      expect(req.request.method).toBe('PUT');
      req.flush({ id: 'w1', merchant_id: 'm1', url: 'https://updated.test', events: [], status: 'active', secret: '', created_at: '' });
    });
  });

  describe('deleteWebhook', () => {
    it('should DELETE /api/merchants/:merchantId/webhooks/:id', () => {
      service.deleteWebhook('m1', 'w1').subscribe();
      const req = httpMock.expectOne('/api/merchants/m1/webhooks/w1');
      expect(req.request.method).toBe('DELETE');
      req.flush(null);
    });
  });
});
