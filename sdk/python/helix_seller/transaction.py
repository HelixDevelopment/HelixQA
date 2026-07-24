from typing import List, Optional
from .client import HelixClient


class TransactionService:
    def __init__(self, client: HelixClient):
        self.client = client

    def process_payment(
        self,
        merchant_id: str,
        customer_id: str,
        payment_method_id: str,
        amount: int,
        currency: str,
        idempotency_key: Optional[str] = None,
    ) -> dict:
        return self.client.post(
            f"/api/v1/merchants/{merchant_id}/transactions",
            {
                "customer_id": customer_id,
                "payment_method_id": payment_method_id,
                "amount": amount,
                "currency": currency,
                **({"idempotency_key": idempotency_key} if idempotency_key else {}),
            },
        )

    def get(self, merchant_id: str, transaction_id: str) -> dict:
        return self.client.get(
            f"/api/v1/merchants/{merchant_id}/transactions/{transaction_id}"
        )

    def list(self, merchant_id: str) -> List[dict]:
        result = self.client.get(
            f"/api/v1/merchants/{merchant_id}/transactions"
        )
        return result.get("transactions", [])

    def refund(
        self,
        merchant_id: str,
        transaction_id: str,
        amount: int,
        reason: Optional[str] = None,
    ) -> dict:
        return self.client.post(
            f"/api/v1/merchants/{merchant_id}/refunds",
            {
                "transaction_id": transaction_id,
                "amount": amount,
                **({"reason": reason} if reason else {}),
            },
        )
