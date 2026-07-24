from typing import Dict
from .client import HelixClient


class AuthService:
    def __init__(self, client: HelixClient):
        self.client = client

    def login(self, email: str, password: str) -> Dict[str, str]:
        tokens = self.client.post(
            "/api/v1/auth/login",
            {"email": email, "password": password},
        )
        self.client.set_api_key(tokens["access_token"])
        return tokens

    def register(
        self, email: str, password: str, name: str
    ) -> Dict[str, str]:
        tokens = self.client.post(
            "/api/v1/auth/register",
            {"email": email, "password": password, "name": name},
        )
        self.client.set_api_key(tokens["access_token"])
        return tokens

    def refresh(self, refresh_token: str) -> Dict[str, str]:
        tokens = self.client.post(
            "/api/v1/auth/refresh",
            {"refresh_token": refresh_token},
        )
        self.client.set_api_key(tokens["access_token"])
        return tokens
