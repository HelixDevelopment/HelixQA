import { HelixClient } from './client';
import { Subscription, CreateSubscriptionRequest } from './types';

export class SubscriptionService {
  constructor(private client: HelixClient) {}

  async create(merchantId: string, request: CreateSubscriptionRequest): Promise<Subscription> {
    return this.client.post<Subscription>(
      `/api/v1/merchants/${merchantId}/subscriptions`,
      request
    );
  }

  async get(merchantId: string, subscriptionId: string): Promise<Subscription> {
    return this.client.get<Subscription>(
      `/api/v1/merchants/${merchantId}/subscriptions/${subscriptionId}`
    );
  }

  async list(merchantId: string): Promise<Subscription[]> {
    const result = await this.client.get<{ subscriptions: Subscription[] }>(
      `/api/v1/merchants/${merchantId}/subscriptions`
    );
    return result.subscriptions;
  }

  async cancel(merchantId: string, subscriptionId: string): Promise<void> {
    await this.client.delete(
      `/api/v1/merchants/${merchantId}/subscriptions/${subscriptionId}`
    );
  }
}
