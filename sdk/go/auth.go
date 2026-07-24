package helixsdk

import (
	"context"
	"encoding/json"
	"fmt"
)

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (c *Client) Login(ctx context.Context, req *LoginRequest) (*AuthTokens, error) {
	data, err := c.do(ctx, "POST", "/api/v1/auth/login", req)
	if err != nil {
		return nil, err
	}
	var tokens AuthTokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &tokens, nil
}

func (c *Client) Register(ctx context.Context, req *RegisterRequest) (*AuthTokens, error) {
	data, err := c.do(ctx, "POST", "/api/v1/auth/register", req)
	if err != nil {
		return nil, err
	}
	var tokens AuthTokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &tokens, nil
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	req := struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: refreshToken}
	data, err := c.do(ctx, "POST", "/api/v1/auth/refresh", req)
	if err != nil {
		return nil, err
	}
	var tokens AuthTokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &tokens, nil
}
