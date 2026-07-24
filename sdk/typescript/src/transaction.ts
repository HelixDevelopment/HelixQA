import { HelixClient } from './client';
import { Transaction, ProcessPaymentRequest } from './types';

export class TransactionService {
  constructor(private client: HelixClient) {}

  async processPayment(merchantId: string, request: ProcessPaymentRequest): Promise<Transaction> {
    return this.client.post<Transaction>(
      `/api/v1/merchants/${merchantId}/transactions`,
      request
    );
  }

  async get(merchantId: string, transactionId: string): Promise<Transaction> {
    return this.client.get<Transaction>(
      `/api/v1/merchants/${merchantId}/transactions/${transactionId}`
    );
  }

  async list(merchantId: string): Promise<Transaction[]> {
    const result = await this.client.get<{ transactions: Transaction[] }>(
      `/api/v1/merchants/${merchantId}/transactions`
    );
    return result.transactions;
  }

  async refund(merchantId: string, transactionId: string, amount: number, reason?: string): Promise<Transaction> {
    return this.client.post<Transaction>(
      `/api/v1/merchants/${merchantId}/refunds`,
      { transaction_id: transactionId, amount, reason }
    );
  }
}
