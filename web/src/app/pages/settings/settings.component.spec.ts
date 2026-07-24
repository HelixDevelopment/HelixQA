import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { SettingsComponent } from './settings.component';

describe('SettingsComponent', () => {
  let component: SettingsComponent;
  let fixture: ComponentFixture<SettingsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [provideRouter([])]
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('should render heading', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('h1')?.textContent).toContain('Settings');
  });

  it('should show placeholder message', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.placeholder')?.textContent).toContain('Settings page coming soon');
  });

  it('should have a settings card', () => {
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.settings-card')).toBeTruthy();
  });
});
