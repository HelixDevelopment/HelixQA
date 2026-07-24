import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { WebhooksComponent } from './webhooks.component';

describe('WebhooksComponent', () => {
  let component: WebhooksComponent;
  let fixture: ComponentFixture<WebhooksComponent>;
  let httpMock: HttpTestingController;

  const mockWebhooks = [
    { id: 'w1', merchant_id: 'default', url: 'https://hook1.test', events: ['payment.succeeded', 'payment.failed'], status: 'active', secret: 'sec1', created_at: '2026-01-01T00:00:00Z' },
    { id: 'w2', merchant_id: 'default', url: 'https://hook2.test', events: ['subscription.created'], status: 'inactive', secret: 'sec2', created_at: '2026-03-01T00:00:00Z' },
  ];

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [WebhooksComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(WebhooksComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component).toBeTruthy();
  });

  it('should load webhooks on init', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });
    expect(component.webhooks.length).toBe(2);
    expect(component.loading).toBeFalse();
  });

  it('should set loading false on error', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush('error', { status: 500, statusText: 'Error' });
    expect(component.loading).toBeFalse();
  });

  it('should render webhook rows', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const rows = el.querySelectorAll('tbody tr');
    expect(rows.length).toBe(2);
  });

  it('should display webhook URLs', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('https://hook1.test');
    expect(el.textContent).toContain('https://hook2.test');
  });

  it('should display event tags', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const eventTags = el.querySelectorAll('.event-tag');
    expect(eventTags.length).toBe(3);
    expect(eventTags[0]?.textContent?.trim()).toBe('payment.succeeded');
    expect(eventTags[1]?.textContent?.trim()).toBe('payment.failed');
    expect(eventTags[2]?.textContent?.trim()).toBe('subscription.created');
  });

  it('should display status badge', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges[0]?.textContent?.trim()).toBe('active');
    expect(badges[1]?.textContent?.trim()).toBe('inactive');
  });

  it('should toggle form visibility', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
    expect(component.showForm).toBeFalse();

    component.showForm = true;
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.form-card')).toBeTruthy();

    component.showForm = false;
    fixture.detectChanges();
    expect(el.querySelector('.form-card')).toBeNull();
  });

  it('should parse comma-separated events on submit', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newWebhook = { url: 'https://newhook.test' };
    component.eventsInput = 'payment.succeeded, subscription.created, refund.failed';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/webhooks');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(jasmine.objectContaining({
      url: 'https://newhook.test',
      events: ['payment.succeeded', 'subscription.created', 'refund.failed']
    }));
    req.flush({ id: 'w3', merchant_id: 'default', url: 'https://newhook.test', events: [], status: 'active', secret: '', created_at: '' });
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    expect(component.showForm).toBeFalse();
    expect(component.eventsInput).toBe('');
  }));

  it('should trim whitespace from events', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newWebhook = { url: 'https://test.com' };
    component.eventsInput = '  event1  ,  event2  ';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/webhooks');
    const body = req.request.body as any;
    expect(body.events).toEqual(['event1', 'event2']);
    req.flush({ id: 'w4', merchant_id: 'default', url: '', events: [], status: 'active', secret: '', created_at: '' });
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
  }));

  it('should filter empty events from parsing', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newWebhook = { url: 'https://test.com' };
    component.eventsInput = 'event1,,event2,';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/webhooks');
    const body = req.request.body as any;
    expect(body.events).toEqual(['event1', 'event2']);
    req.flush({ id: 'w5', merchant_id: 'default', url: '', events: [], status: 'active', secret: '', created_at: '' });
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
  }));

  it('should reset submitting on create error', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    component.showForm = true;
    component.newWebhook = { url: 'https://test.com' };
    component.eventsInput = 'evt1';
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants/default/webhooks');
    req.flush('error', { status: 400, statusText: 'Bad Request' });
    tick();
    expect(component.submitting).toBeFalse();
  }));

  it('should delete webhook after confirm', fakeAsync(() => {
    spyOn(window, 'confirm').and.returnValue(true);
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });

    component.onDelete('w1');
    const req = httpMock.expectOne('/api/merchants/default/webhooks/w1');
    expect(req.request.method).toBe('DELETE');
    req.flush(null);
    tick();

    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });

    expect(window.confirm).toHaveBeenCalled();
  }));

  it('should not delete webhook when confirm is cancelled', () => {
    spyOn(window, 'confirm').and.returnValue(false);
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: mockWebhooks, total: 2, page: 1, per_page: 20 });

    component.onDelete('w1');

    const unmatched = httpMock.match(r => r.url === '/api/merchants/default/webhooks/w1');
    expect(unmatched.length).toBe(0);
  });

  it('should show empty state when no webhooks', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.empty-state')?.textContent).toContain('No webhooks configured');
  });

  it('should have page heading', () => {
    fixture.detectChanges();
    httpMock.expectOne(r => r.url === '/api/merchants/default/webhooks').flush({ data: [], total: 0, page: 1, per_page: 20 });
    fixture.detectChanges();
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Webhook Configurations');
  });
});
