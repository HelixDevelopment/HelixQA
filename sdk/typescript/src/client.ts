import axios, { AxiosInstance, AxiosError } from 'axios';

export interface ClientConfig {
  baseURL: string;
  apiKey?: string;
  timeout?: number;
}

export class APIError extends Error {
  constructor(
    public statusCode: number,
    public body: string
  ) {
    super(`API error ${statusCode}: ${body}`);
    this.name = 'APIError';
  }
}

export class HelixClient {
  private http: AxiosInstance;

  constructor(config: ClientConfig) {
    this.http = axios.create({
      baseURL: config.baseURL,
      timeout: config.timeout || 30000,
      headers: {
        'Content-Type': 'application/json',
        ...(config.apiKey && { Authorization: `Bearer ${config.apiKey}` }),
      },
    });

    this.http.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        if (error.response) {
          throw new APIError(
            error.response.status,
            JSON.stringify(error.response.data)
          );
        }
        throw error;
      }
    );
  }

  setApiKey(key: string) {
    this.http.defaults.headers.common['Authorization'] = `Bearer ${key}`;
  }

  async get<T>(path: string): Promise<T> {
    const response = await this.http.get<T>(path);
    return response.data;
  }

  async post<T>(path: string, data?: unknown): Promise<T> {
    const response = await this.http.post<T>(path, data);
    return response.data;
  }

  async put<T>(path: string, data?: unknown): Promise<T> {
    const response = await this.http.put<T>(path, data);
    return response.data;
  }

  async patch<T>(path: string, data?: unknown): Promise<T> {
    const response = await this.http.patch<T>(path, data);
    return response.data;
  }

  async delete<T>(path: string): Promise<T> {
    const response = await this.http.delete<T>(path);
    return response.data;
  }
}
