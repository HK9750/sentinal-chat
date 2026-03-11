package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	sentinal_errors "sentinal-chat/pkg/errors"
)

type GitHubOAuthClient struct {
	httpClient    *http.Client
	clientID      string
	clientSecret  string
	redirectURI   string
	authorizeURL  string
	tokenEndpoint string
	userEndpoint  string
	emailEndpoint string
}

type githubTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type githubUserResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func NewGitHubOAuthClient(clientID, clientSecret, redirectURI string) (*GitHubOAuthClient, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("github oauth config is incomplete")
	}

	return &GitHubOAuthClient{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		clientID:      strings.TrimSpace(clientID),
		clientSecret:  strings.TrimSpace(clientSecret),
		redirectURI:   strings.TrimSpace(redirectURI),
		authorizeURL:  "https://github.com/login/oauth/authorize",
		tokenEndpoint: "https://github.com/login/oauth/access_token",
		userEndpoint:  "https://api.github.com/user",
		emailEndpoint: "https://api.github.com/user/emails",
	}, nil
}

func (c *GitHubOAuthClient) AuthorizationURL(input OAuthAuthorizeInput) (OAuthAuthorizeResult, error) {
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
	query.Set("scope", "read:user user:email")
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	if state := strings.TrimSpace(input.State); state != "" {
		query.Set("state", state)
	}

	return OAuthAuthorizeResult{
		Provider:         AuthProviderGitHub,
		AuthorizationURL: c.authorizeURL + "?" + query.Encode(),
		RedirectURI:      redirectURI,
		State:            strings.TrimSpace(input.State),
	}, nil
}

func (c *GitHubOAuthClient) ExchangeCode(ctx context.Context, input OAuthExchangeInput) (OAuthIdentity, error) {
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
	form.Set("code_verifier", strings.TrimSpace(input.CodeVerifier))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return OAuthIdentity{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return OAuthIdentity{}, err
	}
	defer res.Body.Close()

	var token githubTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&token); err != nil {
		return OAuthIdentity{}, err
	}
	if res.StatusCode >= 400 || strings.TrimSpace(token.AccessToken) == "" {
		return OAuthIdentity{}, sentinal_errors.ErrUnauthorized
	}

	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.userEndpoint, nil)
	if err != nil {
		return OAuthIdentity{}, err
	}
	userReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")

	userRes, err := c.httpClient.Do(userReq)
	if err != nil {
		return OAuthIdentity{}, err
	}
	defer userRes.Body.Close()

	var profile githubUserResponse
	if err := json.NewDecoder(userRes.Body).Decode(&profile); err != nil {
		return OAuthIdentity{}, err
	}
	if userRes.StatusCode >= 400 || profile.ID == 0 {
		return OAuthIdentity{}, sentinal_errors.ErrUnauthorized
	}

	email, verified, err := c.resolvePrimaryEmail(ctx, token.AccessToken, profile.Email)
	if err != nil {
		return OAuthIdentity{}, err
	}

	return OAuthIdentity{
		Provider:       AuthProviderGitHub,
		ProviderUserID: strconv.FormatInt(profile.ID, 10),
		Email:          strings.ToLower(strings.TrimSpace(email)),
		EmailVerified:  verified,
		DisplayName:    strings.TrimSpace(profile.Name),
		AvatarURL:      strings.TrimSpace(profile.AvatarURL),
	}, nil
}

func (c *GitHubOAuthClient) resolvePrimaryEmail(ctx context.Context, accessToken, fallback string) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.emailEndpoint, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		fallback = strings.TrimSpace(fallback)
		if fallback == "" {
			return "", false, sentinal_errors.ErrUnauthorized
		}
		return fallback, false, nil
	}

	var emails []githubEmail
	if err := json.NewDecoder(res.Body).Decode(&emails); err != nil {
		return "", false, err
	}

	for _, e := range emails {
		if e.Primary {
			return strings.TrimSpace(e.Email), e.Verified, nil
		}
	}
	if len(emails) > 0 {
		return strings.TrimSpace(emails[0].Email), emails[0].Verified, nil
	}

	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return "", false, sentinal_errors.ErrUnauthorized
	}

	return fallback, false, nil
}
