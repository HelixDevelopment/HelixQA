import { HelixClient } from './client';
import { Customer, CreateCustomerRequest } from './types';

export class CustomerService {
  constructor(private client: HelixClient) {}

  async list(merchantId: string): Promise<Customer[]> {
    const result = await this.client.get<{ customers: Customer[] }>(
      `/api/v1/merchants/${merchantId}/customers`
    );
    return result.customers;
  }

  async get(merchantId: string, customerId: string): Promise<Customer> {
    return this.client.get<Customer>(
      `/api/v1/merchants/${merchantId}/customers/${customerId}`
    );
  }

  async create(merchantId: string, request: CreateCustomerRequest): Promise<Customer> {
    return this.client.post<Customer>(
      `/api/v1/merchants/${merchantId}/customers`,
      request
    );
  }

  async update(merchantId: string, customerId: string, data: Partial<CreateCustomerRequest>): Promise<Customer> {
    return this.client.put<Customer>(
      `/api/v1/merchants/${merchantId}/customers/${customerId}`,
      data
    );
  }
}
