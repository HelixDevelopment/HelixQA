from typing import List, Optional
from .client import HelixClient


class CustomerService:
    def __init__(self, client: HelixClient):
        self.client = client

    def list(self, merchant_id: str) -> List[dict]:
        result = self.client.get(f"/api/v1/merchants/{merchant_id}/customers")
        return result.get("customers", [])

    def get(self, merchant_id: str, customer_id: str) -> dict:
        return self.client.get(
            f"/api/v1/merchants/{merchant_id}/customers/{customer_id}"
        )

    def create(
        self,
        merchant_id: str,
        name: str,
        email: str,
        phone: Optional[str] = None,
    ) -> dict:
        return self.client.post(
            f"/api/v1/merchants/{merchant_id}/customers",
            {
                "name": name,
                "email": email,
                **({"phone": phone} if phone else {}),
            },
        )

    def update(self, merchant_id: str, customer_id: str, **kwargs) -> dict:
        return self.client.put(
            f"/api/v1/merchants/{merchant_id}/customers/{customer_id}", kwargs
        )
