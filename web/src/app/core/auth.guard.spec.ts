import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { authGuard } from './auth.guard';

describe('authGuard', () => {
  let router: jasmine.SpyObj<Router>;

  beforeEach(() => {
    router = jasmine.createSpyObj('Router', ['createUrlTree']);
    TestBed.configureTestingModule({
      providers: [
        { provide: Router, useValue: router },
      ]
    });
  });

  afterEach(() => {
    localStorage.removeItem('helix_token');
  });

  it('should allow activation when token exists', () => {
    localStorage.setItem('helix_token', 'valid-token');
    const result = TestBed.runInInjectionContext(() =>
      authGuard({} as any, {} as any)
    );
    expect(result).toBeTrue();
  });

  it('should redirect to /login when no token', () => {
    localStorage.removeItem('helix_token');
    const urlTree = { toString: () => '/login' } as any;
    router.createUrlTree.and.returnValue(urlTree);
    const result = TestBed.runInInjectionContext(() =>
      authGuard({} as any, {} as any)
    );
    expect(result).toBe(urlTree);
    expect(router.createUrlTree).toHaveBeenCalledWith(['/login']);
  });

  it('should redirect to /login when token is empty string', () => {
    localStorage.setItem('helix_token', '');
    const urlTree = { toString: () => '/login' } as any;
    router.createUrlTree.and.returnValue(urlTree);
    const result = TestBed.runInInjectionContext(() =>
      authGuard({} as any, {} as any)
    );
    expect(result).toBe(urlTree);
  });
});
