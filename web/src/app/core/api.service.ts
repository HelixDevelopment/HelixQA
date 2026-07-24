import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export interface Merchant {
  id: string;
  name: string;
  trade_name?: string;
  email: string;
  country?: string;
  currency?: string;
  status: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  merchant_id: string;
  amount: number;
  currency: string;
  type: string;
  provider: string;
  status: string;
  created_at: string;
}

export interface Customer {
  id: string;
  email: string;
  name: string;
  phone?: string;
  external_id?: string;
  created_at: string;
}

export interface Subscription {
  id: string;
  merchant_id: string;
  customer_id: string;
  plan_id: string;
  amount: number;
  interval: string;
  interval_count: number;
  status: string;
  current_period_start: string;
  current_period_end: string;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
}

export interface ProviderConfig {
  id: string;
  merchant_id: string;
  provider: string;
  status: string;
  config: Record<string, unknown>;
  created_at: string;
}

export interface WebhookConfig {
  id: string;
  merchant_id: string;
  url: string;
  events: string[];
  status: string;
  secret: string;
  created_at: string;
}

@Injectable({ providedIn: 'root' })
export class ApiService {
  private http = inject(HttpClient);
  private baseUrl = '/api';

  login(credentials: LoginRequest): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${this.baseUrl}/auth/login`, credentials);
  }

  logout(): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/auth/logout`, {});
  }

  getDashboard(): Observable<Record<string, unknown>> {
    return this.http.get<Record<string, unknown>>(`${this.baseUrl}/dashboard`);
  }

  getMerchants(page = 1, perPage = 20): Observable<PaginatedResponse<Merchant>> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    return this.http.get<PaginatedResponse<Merchant>>(`${this.baseUrl}/merchants`, { params });
  }

  getMerchant(id: string): Observable<Merchant> {
    return this.http.get<Merchant>(`${this.baseUrl}/merchants/${id}`);
  }

  createMerchant(merchant: Partial<Merchant>): Observable<Merchant> {
    return this.http.post<Merchant>(`${this.baseUrl}/merchants`, merchant);
  }

  updateMerchant(id: string, merchant: Partial<Merchant>): Observable<Merchant> {
    return this.http.put<Merchant>(`${this.baseUrl}/merchants/${id}`, merchant);
  }

  deleteMerchant(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/merchants/${id}`);
  }

  getTransactions(page = 1, perPage = 20, merchantId?: string): Observable<PaginatedResponse<Transaction>> {
    let params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    if (merchantId) {
      params = params.set('merchant_id', merchantId);
    }
    return this.http.get<PaginatedResponse<Transaction>>(`${this.baseUrl}/transactions`, { params });
  }

  getTransaction(id: string): Observable<Transaction> {
    return this.http.get<Transaction>(`${this.baseUrl}/transactions/${id}`);
  }

  getCustomers(page = 1, perPage = 20): Observable<PaginatedResponse<Customer>> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    return this.http.get<PaginatedResponse<Customer>>(`${this.baseUrl}/customers`, { params });
  }

  getCustomer(id: string): Observable<Customer> {
    return this.http.get<Customer>(`${this.baseUrl}/customers/${id}`);
  }

  getSubscriptions(page = 1, perPage = 20): Observable<PaginatedResponse<Subscription>> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    return this.http.get<PaginatedResponse<Subscription>>(`${this.baseUrl}/subscriptions`, { params });
  }

  getSubscription(id: string): Observable<Subscription> {
    return this.http.get<Subscription>(`${this.baseUrl}/subscriptions/${id}`);
  }

  getSettings(): Observable<Record<string, unknown>> {
    return this.http.get<Record<string, unknown>>(`${this.baseUrl}/settings`);
  }

  updateSettings(settings: Record<string, unknown>): Observable<Record<string, unknown>> {
    return this.http.put<Record<string, unknown>>(`${this.baseUrl}/settings`, settings);
  }

  getAnalyticsSummary(): Observable<Record<string, unknown>> {
    return this.http.get<Record<string, unknown>>(`${this.baseUrl}/analytics/summary`);
  }

  getProviders(merchantId: string, page = 1, perPage = 20): Observable<PaginatedResponse<ProviderConfig>> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    return this.http.get<PaginatedResponse<ProviderConfig>>(
      `${this.baseUrl}/merchants/${merchantId}/providers`, { params }
    );
  }

  getProvider(merchantId: string, id: string): Observable<ProviderConfig> {
    return this.http.get<ProviderConfig>(`${this.baseUrl}/merchants/${merchantId}/providers/${id}`);
  }

  createProvider(merchantId: string, provider: Partial<ProviderConfig>): Observable<ProviderConfig> {
    return this.http.post<ProviderConfig>(`${this.baseUrl}/merchants/${merchantId}/providers`, provider);
  }

  updateProvider(merchantId: string, id: string, provider: Partial<ProviderConfig>): Observable<ProviderConfig> {
    return this.http.put<ProviderConfig>(`${this.baseUrl}/merchants/${merchantId}/providers/${id}`, provider);
  }

  deleteProvider(merchantId: string, id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/merchants/${merchantId}/providers/${id}`);
  }

  getWebhooks(merchantId: string, page = 1, perPage = 20): Observable<PaginatedResponse<WebhookConfig>> {
    const params = new HttpParams()
      .set('page', page.toString())
      .set('per_page', perPage.toString());
    return this.http.get<PaginatedResponse<WebhookConfig>>(
      `${this.baseUrl}/merchants/${merchantId}/webhooks`, { params }
    );
  }

  getWebhook(merchantId: string, id: string): Observable<WebhookConfig> {
    return this.http.get<WebhookConfig>(`${this.baseUrl}/merchants/${merchantId}/webhooks/${id}`);
  }

  createWebhook(merchantId: string, webhook: Partial<WebhookConfig>): Observable<WebhookConfig> {
    return this.http.post<WebhookConfig>(`${this.baseUrl}/merchants/${merchantId}/webhooks`, webhook);
  }

  updateWebhook(merchantId: string, id: string, webhook: Partial<WebhookConfig>): Observable<WebhookConfig> {
    return this.http.put<WebhookConfig>(`${this.baseUrl}/merchants/${merchantId}/webhooks/${id}`, webhook);
  }

  deleteWebhook(merchantId: string, id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/merchants/${merchantId}/webhooks/${id}`);
  }
}
