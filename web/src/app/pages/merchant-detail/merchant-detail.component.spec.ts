import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { provideRouter, ActivatedRoute } from '@angular/router';
import { MerchantDetailComponent } from './merchant-detail.component';

describe('MerchantDetailComponent', () => {
  let component: MerchantDetailComponent;
  let fixture: ComponentFixture<MerchantDetailComponent>;
  let httpMock: HttpTestingController;

  const mockMerchant = {
    id: 'm1',
    name: 'Acme Corp',
    legal_name: 'Acme Corp',
    trade_name: 'Acme Store',
    email: 'acme@test.com',
    country: 'US',
    currency: 'USD',
    status: 'active',
    kyc_status: 'verified',
    phone: '+1234567890',
    timezone: 'America/New_York',
    created_at: '2026-01-15T10:00:00Z',
    updated_at: '2026-06-20T14:30:00Z'
  };

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [MerchantDetailComponent],
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
    fixture = TestBed.createComponent(MerchantDetailComponent);
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

  it('should load merchant on init', () => {
    fixture.detectChanges();
    const req = httpMock.expectOne('/api/merchants/m1');
    req.flush(mockMerchant);

    expect(component.merchant).toBeTruthy();
    expect(component.merchant.name).toBe('Acme Corp');
    expect(component.merchant.trade_name).toBe('Acme Store');
  });

  it('should render merchant trade_name as heading', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Acme Store');
  });

  it('should render merchant details in dl elements', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    expect(el.textContent).toContain('Acme Corp');
    expect(el.textContent).toContain('acme@test.com');
    expect(el.textContent).toContain('US');
    expect(el.textContent).toContain('USD');
  });

  it('should render status and kyc badges', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const badges = el.querySelectorAll('.badge');
    expect(badges.length).toBe(2);
    expect(badges[0]?.textContent?.trim()).toBe('active');
    expect(badges[1]?.textContent?.trim()).toBe('verified');
  });

  it('should handle API error gracefully', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush('error', { status: 404, statusText: 'Not Found' });

    expect(component.merchant).toBeNull();
    expect(component.error).toBeTruthy();
    expect(component.loading).toBeFalse();
  });

  it('should render back link to merchants list', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const link = el.querySelector('a.back-link');
    expect(link?.getAttribute('href')).toBe('/merchants');
  });

  it('should render edit button linking to edit page', () => {
    fixture.detectChanges();
    httpMock.expectOne('/api/merchants/m1').flush(mockMerchant);
    fixture.detectChanges();

    const el: HTMLElement = fixture.nativeElement;
    const editLink = el.querySelector('a[routerLink]');
    expect(editLink).toBeTruthy();
  });
});
