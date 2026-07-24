import { ComponentFixture, TestBed, fakeAsync, tick } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter, ActivatedRoute, Router, Route } from '@angular/router';
import { MerchantEditComponent } from './merchant-edit.component';

describe('MerchantEditComponent', () => {
  let component: MerchantEditComponent;
  let fixture: ComponentFixture<MerchantEditComponent>;
  let httpMock: HttpTestingController;

  const mockMerchant = {
    id: 'm1',
    name: 'Acme Corp',
    legal_name: 'Acme Corp',
    trade_name: 'Acme Store',
    email: 'acme@test.com',
    phone: '+1234567890',
    country: 'US',
    currency: 'USD',
    status: 'active',
    created_at: '2026-01-15T10:00:00Z',
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MerchantEditComponent],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: {
              paramMap: {
                get: (key: string) => key === 'id' ? 'm1' : null,
              }
            }
          }
        }
      ]
    }).compileComponents();

    httpMock = TestBed.inject(HttpTestingController);
    fixture = TestBed.createComponent(MerchantEditComponent);
    component = fixture.componentInstance;
  });

  afterEach(() => {
    httpMock.verify();
  });

  it('should create', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    expect(component).toBeTruthy();
  });

  it('should not render form before data loads', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/merchants/m1');
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.form-card')).toBeNull();
    req.flush(mockMerchant);
  });

  it('should load merchant data into form on init', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);

    expect(component.loaded).toBeTrue();
    expect(component.formData.legal_name).toBe('Acme Corp');
    expect(component.formData.email).toBe('acme@test.com');
    expect(component.formData.trade_name).toBe('Acme Store');
    expect(component.formData.phone).toBe('+1234567890');
    expect(component.formData.country).toBe('US');
    expect(component.formData.currency).toBe('USD');
  });

  it('should render form fields after loading', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('#legal_name')).toBeTruthy();
    expect(el.querySelector('#email')).toBeTruthy();
    expect(el.querySelector('#trade_name')).toBeTruthy();
    expect(el.querySelector('#phone')).toBeTruthy();
    expect(el.querySelector('#country')).toBeTruthy();
    expect(el.querySelector('#currency')).toBeTruthy();
  });

  it('should call updateMerchant on submit', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);

    const router = TestBed.inject(Router);
    spyOn(router, 'navigate').and.resolveTo(true);

    component.formData.legal_name = 'Updated Name';
    component.onSubmit();

    expect(component.submitting).toBeTrue();
    const req = httpMock.expectOne('/api/merchants/m1');
    expect(req.request.method).toBe('PUT');
    expect(req.request.body).toEqual(component.formData);
    req.flush({ id: 'm1', ...component.formData, status: 'active', created_at: '' });
    tick();
  }));

  it('should reset submitting on update error', fakeAsync(() => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);

    component.onSubmit();
    const req = httpMock.expectOne('/api/merchants/m1');
    req.flush('error', { status: 400, statusText: 'Bad Request' });
    tick();

    expect(component.submitting).toBeFalse();
  }));

  it('should navigate to /merchants on load error', fakeAsync(() => {
    const router = TestBed.inject(Router);
    const routerSpy = spyOn(router, 'navigate').and.resolveTo(true);
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush('error', { status: 404, statusText: 'Not Found' });
    tick();

    expect(routerSpy).toHaveBeenCalledWith(['/merchants']);
  }));

  it('should show saving text on submit button when submitting', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);

    component.submitting = true;
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const button = el.querySelector('button[type="submit"]');
    expect(button?.textContent?.trim()).toBe('Saving...');
  });
});
