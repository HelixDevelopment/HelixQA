export interface Merchant {
  id: string;
  legal_name: string;
  trade_name: string;
  email: string;
  phone: string;
  country: string;
  currency: string;
  status: string;
  created_at: string;
}

export interface Customer {
  id: string;
  merchant_id: string;
  name: string;
  email: string;
  phone: string;
  created_at: string;
}

export interface Transaction {
  id: string;
  merchant_id: string;
  customer_id: string;
  amount: number;
  currency: string;
  status: string;
  provider: string;
  created_at: string;
}

export interface Subscription {
  id: string;
  merchant_id: string;
  customer_id: string;
  amount: number;
  currency: string;
  status: string;
  interval: string;
  created_at: string;
}

export interface AuthTokens {
  access_token: string;
  refresh_token: string;
}

export interface CreateMerchantRequest {
  legal_name: string;
  trade_name?: string;
  email: string;
  phone?: string;
  country: string;
  currency: string;
}

export interface CreateCustomerRequest {
  name: string;
  email: string;
  phone?: string;
}

export interface ProcessPaymentRequest {
  customer_id: string;
  payment_method_id: string;
  amount: number;
  currency: string;
  idempotency_key?: string;
}

export interface CreateSubscriptionRequest {
  customer_id: string;
  amount: number;
  currency: string;
  interval: string;
  interval_count?: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}
