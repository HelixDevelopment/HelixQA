export { HelixClient, ClientConfig, APIError } from './client';
export { MerchantService } from './merchant';
export { CustomerService } from './customer';
export { TransactionService } from './transaction';
export { SubscriptionService } from './subscription';
export { AuthService } from './auth';
export * from './types';

import { HelixClient, ClientConfig } from './client';
import { MerchantService } from './merchant';
import { CustomerService } from './customer';
import { TransactionService } from './transaction';
import { SubscriptionService } from './subscription';
import { AuthService } from './auth';

export function createHelixClient(config: ClientConfig) {
  const client = new HelixClient(config);
  return {
    client,
    merchants: new MerchantService(client),
    customers: new CustomerService(client),
    transactions: new TransactionService(client),
    subscriptions: new SubscriptionService(client),
    auth: new AuthService(client),
  };
}
