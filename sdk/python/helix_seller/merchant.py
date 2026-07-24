from typing import List, Optional
from .client import HelixClient


class MerchantService:
    def __init__(self, client: HelixClient):
        self.client = client

    def list(self) -> List[dict]:
        result = self.client.get("/api/v1/merchants")
        return result.get("merchants", [])

    def get(self, merchant_id: str) -> dict:
        return self.client.get(f"/api/v1/merchants/{merchant_id}")

    def create(
        self,
        legal_name: str,
        email: str,
        country: str,
        currency: str,
        trade_name: Optional[str] = None,
        phone: Optional[str] = None,
    ) -> dict:
        return self.client.post(
            "/api/v1/merchants",
            {
                "legal_name": legal_name,
                "email": email,
                "country": country,
                "currency": currency,
                **({"trade_name": trade_name} if trade_name else {}),
                **({"phone": phone} if phone else {}),
            },
        )

    def update(self, merchant_id: str, **kwargs) -> dict:
        return self.client.put(f"/api/v1/merchants/{merchant_id}", kwargs)
