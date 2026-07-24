import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { MerchantCreateComponent } from './merchant-create.component';

describe('MerchantCreateComponent', () => {
  let component: MerchantCreateComponent;
  let fixture: ComponentFixture<MerchantCreateComponent>;
  let httpMock: HttpTestingController;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MerchantCreateComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(MerchantCreateComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should have empty form data initially', () => {
    expect(component.formData.name).toBe('');
    expect(component.formData.email).toBe('');
    expect(component.formData.trade_name).toBe('');
    expect(component.formData.country).toBe('');
    expect(component.formData.currency).toBe('');
    expect(component.submitting).toBeFalse();
  });

  it('should render form fields', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('#name')).toBeTruthy();
    expect(el.querySelector('#email')).toBeTruthy();
    expect(el.querySelector('#trade_name')).toBeTruthy();
    expect(el.querySelector('#country')).toBeTruthy();
    expect(el.querySelector('#currency')).toBeTruthy();
  });

  it('should render heading', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Create Merchant');
  });

  it('should render back link', () => {
    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a.back-link');
    expect(link).toBeTruthy();
    expect(link?.getAttribute('href')).toBe('/merchants');
  });

  it('should render submit button', () => {
    const el: HTMLElement = fixture.nativeElement;
    const button = el.querySelector('button[type="submit"]');
    expect(button?.textContent?.trim()).toBe('Create Merchant');
  });

  it('should call API on submit and navigate', fakeAsync(() => {
    const routerSpy = spyOn(TestBed.inject(Router), 'navigate');

    component.formData = { name: 'New Shop', email: 'shop@test.com', trade_name: 'New', country: 'US', currency: 'USD' };
    component.onSubmit();

    expect(component.submitting).toBeTrue();
    const req = httpMock.expectOne('/api/merchants');
    expect(req.request.method).toBe('POST');
    expect(req.request.body).toEqual(component.formData);
    req.flush({ id: 'm3', ...component.formData, status: 'active', created_at: '' });
    tick();

    expect(routerSpy).toHaveBeenCalledWith(['/merchants']);
  }));

  it('should reset submitting on error', fakeAsync(() => {
    component.formData = { name: 'Fail', email: 'f@t.com', trade_name: '', country: 'US', currency: 'USD' };
    component.onSubmit();

    const req = httpMock.expectOne('/api/merchants');
    req.flush('error', { status: 400, statusText: 'Bad Request' });
    tick();

    expect(component.submitting).toBeFalse();
  }));
});
