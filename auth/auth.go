// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/nbutton23/zxcvbn-go"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/params"
)

func NewAuthenticator(cfg config.JWTAuth, store common.Store) *Authenticator {
	return &Authenticator{
		cfg:        cfg,
		store:      store,
		oidcStates: make(map[string]oidcStateEntry),
	}
}

type Authenticator struct {
	store common.Store
	cfg   config.JWTAuth

	// OIDC fields
	oidcCfg      config.OIDC
	oidcProvider *oidc.Provider
	oidcVerifier *oidc.IDTokenVerifier
	oidcOAuth2   oauth2.Config
	oidcStateMu  sync.RWMutex
	oidcStates   map[string]oidcStateEntry
}

type oidcStateEntry struct {
	createdAt time.Time
	nonce     string
}

func (a *Authenticator) IsInitialized() bool {
	return a.store.HasAdminUser(context.Background())
}

func (a *Authenticator) GetJWTToken(ctx context.Context) (string, error) {
	tokenID, err := util.GetRandomString(16)
	if err != nil {
		return "", fmt.Errorf("error generating random string: %w", err)
	}
	expireToken := time.Now().Add(a.cfg.TimeToLive.Duration())
	expires := &jwt.NumericDate{
		Time: expireToken,
	}
	generation := PasswordGeneration(ctx)
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expires,
			// nolint:golangci-lint,godox
			// TODO: make this configurable
			Issuer: "garm",
		},
		UserID:     UserID(ctx),
		TokenID:    tokenID,
		Username:   Username(ctx),
		IsAdmin:    IsAdmin(ctx),
		FullName:   FullName(ctx),
		Generation: generation,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("error fetching token string: %w", err)
	}

	return tokenString, nil
}

// GetJWTMetricsToken returns a JWT token that can be used to read metrics.
// This token is not tied to a user, no user is stored in the db.
func (a *Authenticator) GetJWTMetricsToken(ctx context.Context) (string, error) {
	if !IsAdmin(ctx) {
		return "", runnerErrors.ErrUnauthorized
	}

	tokenID, err := util.GetRandomString(16)
	if err != nil {
		return "", fmt.Errorf("error generating random string: %w", err)
	}
	// nolint:golangci-lint,godox
	// TODO: currently this is the same TTL as the normal Token
	// maybe we should make this configurable
	// it's usually pretty nasty if the monitoring fails because the token expired
	expireToken := time.Now().Add(a.cfg.TimeToLive.Duration())
	expires := &jwt.NumericDate{
		Time: expireToken,
	}
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: expires,
			// nolint:golangci-lint,godox
			// TODO: make this configurable
			Issuer: "garm",
		},
		TokenID:     tokenID,
		IsAdmin:     false,
		ReadMetrics: true,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(a.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("error fetching token string: %w", err)
	}

	return tokenString, nil
}

func (a *Authenticator) InitController(ctx context.Context, param params.NewUserParams) (params.User, error) {
	_, err := a.store.ControllerInfo()
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return params.User{}, fmt.Errorf("error initializing controller: %w", err)
		}
	}
	if a.store.HasAdminUser(ctx) {
		return params.User{}, runnerErrors.ErrNotFound
	}

	if param.Email == "" || param.Username == "" {
		return params.User{}, runnerErrors.NewBadRequestError("missing username or email")
	}

	if !util.IsValidEmail(param.Email) {
		return params.User{}, runnerErrors.NewBadRequestError("invalid email address")
	}

	// username is varchar(64)
	if len(param.Username) > 64 || !util.IsAlphanumeric(param.Username) {
		return params.User{}, runnerErrors.NewBadRequestError("invalid username")
	}

	param.IsAdmin = true
	param.Enabled = true

	passwordStenght := zxcvbn.PasswordStrength(param.Password, nil)
	if passwordStenght.Score < 4 {
		return params.User{}, runnerErrors.NewBadRequestError("password is too weak")
	}

	hashed, err := util.PaswsordToBcrypt(param.Password)
	if err != nil {
		return params.User{}, fmt.Errorf("error creating user: %w", err)
	}

	param.Password = hashed

	return a.store.CreateUser(ctx, param)
}

func (a *Authenticator) AuthenticateUser(ctx context.Context, info params.PasswordLoginParams) (context.Context, error) {
	if info.Username == "" || info.Password == "" {
		return ctx, runnerErrors.ErrUnauthorized
	}

	user, err := a.store.GetUser(ctx, info.Username)
	if err != nil {
		if errors.Is(err, runnerErrors.ErrNotFound) {
			return ctx, runnerErrors.ErrUnauthorized
		}
		return ctx, fmt.Errorf("error authenticating: %w", err)
	}

	if !user.Enabled {
		return ctx, runnerErrors.ErrUnauthorized
	}

	if user.Password == "" {
		return ctx, runnerErrors.ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(info.Password)); err != nil {
		return ctx, runnerErrors.ErrUnauthorized
	}

	return PopulateContext(ctx, user, nil), nil
}

// InitOIDC initializes OIDC authentication
func (a *Authenticator) InitOIDC(ctx context.Context, cfg config.OIDC) error {
	if !cfg.Enable {
		return nil
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	a.oidcCfg = cfg
	a.oidcProvider = provider
	a.oidcVerifier = provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	a.oidcOAuth2 = oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.GetScopes(),
	}

	return nil
}

// IsOIDCEnabled returns whether OIDC is enabled
func (a *Authenticator) IsOIDCEnabled() bool {
	return a.oidcCfg.Enable && a.oidcProvider != nil
}

// GetOIDCAuthURL returns the OIDC authorization URL
func (a *Authenticator) GetOIDCAuthURL() (string, string, error) {
	if !a.IsOIDCEnabled() {
		return "", "", runnerErrors.NewBadRequestError("OIDC authentication is not enabled")
	}

	state, err := a.generateOIDCState()
	if err != nil {
		return "", "", err
	}

	nonce, err := a.generateOIDCNonce()
	if err != nil {
		return "", "", err
	}

	// Store state with expiration
	a.oidcStateMu.Lock()
	a.oidcStates[state] = oidcStateEntry{
		createdAt: time.Now(),
		nonce:     nonce,
	}
	a.oidcStateMu.Unlock()

	// Clean up old states
	go a.cleanupOIDCStates()

	url := a.oidcOAuth2.AuthCodeURL(state, oidc.Nonce(nonce))
	return url, state, nil
}

// generateOIDCState creates a cryptographically secure random state
func (a *Authenticator) generateOIDCState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateOIDCNonce creates a cryptographically secure random nonce
func (a *Authenticator) generateOIDCNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// cleanupOIDCStates removes expired states (older than 10 minutes)
func (a *Authenticator) cleanupOIDCStates() {
	a.oidcStateMu.Lock()
	defer a.oidcStateMu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for state, entry := range a.oidcStates {
		if entry.createdAt.Before(cutoff) {
			delete(a.oidcStates, state)
		}
	}
}

// validateOIDCState checks if the state is valid and returns the nonce
func (a *Authenticator) validateOIDCState(state string) (string, error) {
	a.oidcStateMu.Lock()
	defer a.oidcStateMu.Unlock()

	entry, ok := a.oidcStates[state]
	if !ok {
		return "", runnerErrors.NewBadRequestError("invalid state")
	}

	// Check if state is expired (10 minutes)
	if time.Since(entry.createdAt) > 10*time.Minute {
		delete(a.oidcStates, state)
		return "", runnerErrors.NewBadRequestError("state expired")
	}

	// Delete state after use (one-time use)
	delete(a.oidcStates, state)
	return entry.nonce, nil
}

// OIDCClaims represents the claims from an OIDC ID token
type OIDCClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Subject       string `json:"sub"`
}

// HandleOIDCCallback processes the OIDC callback and returns an authenticated context
func (a *Authenticator) HandleOIDCCallback(ctx context.Context, code, state string) (context.Context, error) {
	if !a.IsOIDCEnabled() {
		return ctx, runnerErrors.NewBadRequestError("OIDC authentication is not enabled")
	}

	// Validate state and get nonce
	nonce, err := a.validateOIDCState(state)
	if err != nil {
		return ctx, err
	}

	// Exchange code for token
	oauth2Token, err := a.oidcOAuth2.Exchange(ctx, code)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("failed to exchange code for token")
		return ctx, runnerErrors.NewBadRequestError("failed to exchange code for token")
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return ctx, runnerErrors.NewBadRequestError("no id_token in token response")
	}

	// Verify ID token
	idToken, err := a.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		slog.With(slog.Any("error", err)).Error("failed to verify ID token")
		return ctx, runnerErrors.NewBadRequestError("failed to verify ID token")
	}

	// Verify nonce
	if idToken.Nonce != nonce {
		return ctx, runnerErrors.NewBadRequestError("nonce mismatch")
	}

	// Extract claims
	var claims OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		slog.With(slog.Any("error", err)).Error("failed to extract claims")
		return ctx, runnerErrors.NewBadRequestError("failed to extract claims")
	}

	// Validate email
	if claims.Email == "" {
		return ctx, runnerErrors.NewBadRequestError("email claim is required")
	}

	// Check allowed domains
	if len(a.oidcCfg.AllowedDomains) > 0 {
		emailDomain := extractEmailDomain(claims.Email)
		allowed := false
		for _, domain := range a.oidcCfg.AllowedDomains {
			if strings.EqualFold(emailDomain, domain) {
				allowed = true
				break
			}
		}
		if !allowed {
			slog.With(slog.String("email", claims.Email)).Warn("email domain not allowed")
			return ctx, runnerErrors.ErrUnauthorized
		}
	}

	// Try to find existing user
	user, err := a.store.GetUser(ctx, claims.Email)
	if err != nil {
		if !errors.Is(err, runnerErrors.ErrNotFound) {
			return ctx, fmt.Errorf("failed to get user: %w", err)
		}

		// User not found - check if JIT creation is enabled
		if !a.oidcCfg.JITUserCreation {
			slog.With(slog.String("email", claims.Email)).Warn("user not found and JIT creation disabled")
			return ctx, runnerErrors.ErrUnauthorized
		}

		// Create user JIT
		user, err = a.createOIDCUser(ctx, claims)
		if err != nil {
			return ctx, fmt.Errorf("failed to create JIT user: %w", err)
		}
		slog.With(slog.String("email", claims.Email)).Info("created JIT user via OIDC")
	}

	// Check if user is enabled
	if !user.Enabled {
		return ctx, runnerErrors.ErrUnauthorized
	}

	return PopulateContext(ctx, user, nil), nil
}

// createOIDCUser creates a new user from OIDC claims
func (a *Authenticator) createOIDCUser(ctx context.Context, claims OIDCClaims) (params.User, error) {
	// Generate username from email (before @)
	username := strings.Split(claims.Email, "@")[0]
	// Sanitize username - only alphanumeric
	username = sanitizeOIDCUsername(username)
	if len(username) > 64 {
		username = username[:64]
	}

	// Use name from claims or fallback to username
	fullName := claims.Name
	if fullName == "" {
		fullName = username
	}

	newUser := params.NewUserParams{
		Email:     claims.Email,
		Username:  username,
		FullName:  fullName,
		Password:  "", // SSO users don't have passwords
		IsAdmin:   a.oidcCfg.DefaultUserAdmin,
		Enabled:   true,
		IsSSOUser: true,
	}

	return a.store.CreateUser(ctx, newUser)
}

// extractEmailDomain extracts the domain from an email address
func extractEmailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// sanitizeOIDCUsername removes non-alphanumeric characters from username
func sanitizeOIDCUsername(s string) string {
	var result strings.Builder
	for _, r := range s {
		if util.IsAlphanumeric(string(r)) {
			result.WriteRune(r)
		}
	}
	return result.String()
}
