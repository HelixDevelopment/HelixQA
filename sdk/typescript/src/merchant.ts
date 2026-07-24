import { HelixClient } from './client';
import { Merchant, CreateMerchantRequest } from './types';

export class MerchantService {
  constructor(private client: HelixClient) {}

  async list(): Promise<Merchant[]> {
    const result = await this.client.get<{ merchants: Merchant[] }>('/api/v1/merchants');
    return result.merchants;
  }

  async get(id: string): Promise<Merchant> {
    return this.client.get<Merchant>(`/api/v1/merchants/${id}`);
  }

  async create(request: CreateMerchantRequest): Promise<Merchant> {
    return this.client.post<Merchant>('/api/v1/merchants', request);
  }

  async update(id: string, data: Partial<CreateMerchantRequest>): Promise<Merchant> {
    return this.client.put<Merchant>(`/api/v1/merchants/${id}`, data);
  }
}
