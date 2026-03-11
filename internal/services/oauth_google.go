package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	sentinal_errors "sentinal-chat/pkg/errors"
)

type GoogleOAuthClient struct {
	httpClient    *http.Client
	clientID      string
	clientSecret  string
	redirectURI   string
	authorizeURL  string
	tokenEndpoint string
	userEndpoint  string
}

type googleTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func NewGoogleOAuthClient(clientID, clientSecret, redirectURI string) (*GoogleOAuthClient, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("google oauth config is incomplete")
	}

	return &GoogleOAuthClient{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		clientID:      strings.TrimSpace(clientID),
		clientSecret:  strings.TrimSpace(clientSecret),
		redirectURI:   strings.TrimSpace(redirectURI),
		authorizeURL:  "https://accounts.google.com/o/oauth2/v2/auth",
		tokenEndpoint: "https://oauth2.googleapis.com/token",
		userEndpoint:  "https://openidconnect.googleapis.com/v1/userinfo",
	}, nil
}

func (c *GoogleOAuthClient) AuthorizationURL(input OAuthAuthorizeInput) (OAuthAuthorizeResult, error) {
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if redirectURI == "" {
		redirectURI = c.redirectURI
	}
	if redirectURI != c.redirectURI {
		return OAuthAuthorizeResult{}, sentinal_errors.ErrInvalidInput
	}

	codeChallenge := strings.TrimSpace(input.CodeChallenge)
	if codeChallenge == "" {
		return OAuthAuthorizeResult{}, sentinal_errors.ErrInvalidInput
	}

	query := url.Values{}
	query.Set("client_id", c.clientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	if state := strings.TrimSpace(input.State); state != "" {
		query.Set("state", state)
	}

	return OAuthAuthorizeResult{
		Provider:         AuthProviderGoogle,
		AuthorizationURL: c.authorizeURL + "?" + query.Encode(),
		RedirectURI:      redirectURI,
		State:            strings.TrimSpace(input.State),
	}, nil
}

func (c *GoogleOAuthClient) ExchangeCode(ctx context.Context, input OAuthExchangeInput) (OAuthIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.CodeVerifier) == "" {
		return OAuthIdentity{}, sentinal_errors.ErrInvalidInput
	}
	if strings.TrimSpace(input.RedirectURI) != c.redirectURI {
		return OAuthIdentity{}, sentinal_errors.ErrInvalidInput
	}

	form := url.Values{}
	form.Set("code", strings.TrimSpace(input.Code))
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("redirect_uri", c.redirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", strings.TrimSpace(input.CodeVerifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthIdentity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthIdentity{}, err
	}
	defer res.Body.Close()

	var token googleTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return OAuthIdentity{}, err
	}
	if res.StatusCode >= 400 || token.AccessToken == "" {
		return OAuthIdentity{}, sentinal_errors.ErrUnauthorized
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userEndpoint, nil)
	if err != nil {
		return OAuthIdentity{}, err
	}
	userReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	userRes, err := c.httpClient.Do(userReq)
	if err != nil {
		return OAuthIdentity{}, err
	}
	defer userRes.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(userRes.Body).Decode(&userInfo); err != nil {
		return OAuthIdentity{}, err
	}
	if userRes.StatusCode >= 400 || strings.TrimSpace(userInfo.Sub) == "" {
		return OAuthIdentity{}, sentinal_errors.ErrUnauthorized
	}

	return OAuthIdentity{
		Provider:       AuthProviderGoogle,
		ProviderUserID: strings.TrimSpace(userInfo.Sub),
		Email:          strings.ToLower(strings.TrimSpace(userInfo.Email)),
		EmailVerified:  userInfo.EmailVerified,
		DisplayName:    strings.TrimSpace(userInfo.Name),
		AvatarURL:      strings.TrimSpace(userInfo.Picture),
	}, nil
}
