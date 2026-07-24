from typing import List, Optional
from .client import HelixClient


class SubscriptionService:
    def __init__(self, client: HelixClient):
        self.client = client

    def create(
        self,
        merchant_id: str,
        customer_id: str,
        amount: int,
        currency: str,
        interval: str,
        interval_count: int = 1,
    ) -> dict:
        return self.client.post(
            f"/api/v1/merchants/{merchant_id}/subscriptions",
            {
                "customer_id": customer_id,
                "amount": amount,
                "currency": currency,
                "interval": interval,
                "interval_count": interval_count,
            },
        )

    def get(self, merchant_id: str, subscription_id: str) -> dict:
        return self.client.get(
            f"/api/v1/merchants/{merchant_id}/subscriptions/{subscription_id}"
        )

    def list(self, merchant_id: str) -> List[dict]:
        result = self.client.get(
            f"/api/v1/merchants/{merchant_id}/subscriptions"
        )
        return result.get("subscriptions", [])

    def cancel(self, merchant_id: str, subscription_id: str) -> None:
        self.client.delete(
            f"/api/v1/merchants/{merchant_id}/subscriptions/{subscription_id}"
        )
