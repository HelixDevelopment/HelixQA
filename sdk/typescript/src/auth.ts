import { HelixClient } from './client';
import { AuthTokens } from './types';

export class AuthService {
  constructor(private client: HelixClient) {}

  async login(email: string, password: string): Promise<AuthTokens> {
    const tokens = await this.client.post<AuthTokens>('/api/v1/auth/login', {
      email,
      password,
    });
    this.client.setApiKey(tokens.access_token);
    return tokens;
  }

  async register(email: string, password: string, name: string): Promise<AuthTokens> {
    const tokens = await this.client.post<AuthTokens>('/api/v1/auth/register', {
      email,
      password,
      name,
    });
    this.client.setApiKey(tokens.access_token);
    return tokens;
  }

  async refresh(refreshToken: string): Promise<AuthTokens> {
    const tokens = await this.client.post<AuthTokens>('/api/v1/auth/refresh', {
      refresh_token: refreshToken,
    });
    this.client.setApiKey(tokens.access_token);
    return tokens;
  }
}
