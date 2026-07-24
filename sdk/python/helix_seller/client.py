from typing import Any, Optional
import httpx


class APIError(Exception):
    def __init__(self, status_code: int, body: str):
        self.status_code = status_code
        self.body = body
        super().__init__(f"API error {status_code}: {body}")


class HelixClient:
    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.http = httpx.Client(
            base_url=self.base_url,
            timeout=timeout,
            headers={
                "Content-Type": "application/json",
                **({"Authorization": f"Bearer {api_key}"} if api_key else {}),
            },
        )

    def set_api_key(self, key: str) -> None:
        self.http.headers["Authorization"] = f"Bearer {key}"

    def _handle_response(self, response: httpx.Response) -> Any:
        if response.status_code >= 400:
            raise APIError(response.status_code, response.text)
        return response.json()

    def get(self, path: str) -> Any:
        response = self.http.get(path)
        return self._handle_response(response)

    def post(self, path: str, data: Optional[dict] = None) -> Any:
        response = self.http.post(path, json=data)
        return self._handle_response(response)

    def put(self, path: str, data: Optional[dict] = None) -> Any:
        response = self.http.put(path, json=data)
        return self._handle_response(response)

    def patch(self, path: str, data: Optional[dict] = None) -> Any:
        response = self.http.patch(path, json=data)
        return self._handle_response(response)

    def delete(self, path: str) -> Any:
        response = self.http.delete(path)
        return self._handle_response(response)

    def close(self) -> None:
        self.http.close()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()
