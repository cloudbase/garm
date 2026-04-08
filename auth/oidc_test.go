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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudbase/garm/config"
)

func TestAuthenticator_IsOIDCEnabled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *Authenticator
		expected bool
	}{
		{
			name: "OIDC not initialized",
			setup: func() *Authenticator {
				return NewAuthenticator(config.JWTAuth{}, nil)
			},
			expected: false,
		},
		{
			name: "OIDC disabled in config",
			setup: func() *Authenticator {
				auth := NewAuthenticator(config.JWTAuth{}, nil)
				auth.oidcCfg = config.OIDC{Enable: false}
				return auth
			},
			expected: false,
		},
		{
			name: "OIDC enabled but provider nil",
			setup: func() *Authenticator {
				auth := NewAuthenticator(config.JWTAuth{}, nil)
				auth.oidcCfg = config.OIDC{Enable: true}
				// provider is nil
				return auth
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := tt.setup()
			result := auth.IsOIDCEnabled()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractEmailDomain(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "valid email",
			email:    "user@example.com",
			expected: "example.com",
		},
		{
			name:     "subdomain email",
			email:    "user@mail.example.com",
			expected: "mail.example.com",
		},
		{
			name:     "no @ symbol",
			email:    "invalid-email",
			expected: "",
		},
		{
			name:     "empty string",
			email:    "",
			expected: "",
		},
		{
			name:     "multiple @ symbols",
			email:    "user@domain@example.com",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailDomain(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeOIDCUsername(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "alphanumeric only",
			input:    "testuser123",
			expected: "testuser123",
		},
		{
			name:     "with dots",
			input:    "test.user",
			expected: "testuser",
		},
		{
			name:     "with special chars",
			input:    "test-user_name+extra",
			expected: "testusernameextra",
		},
		{
			name:     "with spaces",
			input:    "test user",
			expected: "testuser",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only special chars",
			input:    ".-_+@",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOIDCUsername(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuthenticator_GenerateOIDCState(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	state1, err := auth.generateOIDCState()
	require.NoError(t, err)
	assert.NotEmpty(t, state1)

	state2, err := auth.generateOIDCState()
	require.NoError(t, err)
	assert.NotEmpty(t, state2)

	// States should be different
	assert.NotEqual(t, state1, state2)
}

func TestAuthenticator_GenerateOIDCNonce(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	nonce1, err := auth.generateOIDCNonce()
	require.NoError(t, err)
	assert.NotEmpty(t, nonce1)

	nonce2, err := auth.generateOIDCNonce()
	require.NoError(t, err)
	assert.NotEmpty(t, nonce2)

	// Nonces should be different
	assert.NotEqual(t, nonce1, nonce2)
}

func TestAuthenticator_ValidateOIDCState(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	// Add a valid state manually
	testState := "test-state-123"
	testNonce := "test-nonce-456"
	auth.oidcStates[testState] = oidcStateEntry{
		createdAt: time.Now(),
		nonce:     testNonce,
	}

	nonce, err := auth.validateOIDCState(testState)
	require.NoError(t, err)
	assert.Equal(t, testNonce, nonce)

	// State should be consumed (one-time use)
	_, err = auth.validateOIDCState(testState)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}

func TestAuthenticator_ValidateOIDCState_Invalid(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	// Test invalid state
	_, err := auth.validateOIDCState("invalid-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state")
}

func TestAuthenticator_ValidateOIDCState_Expired(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	// Add an expired state manually
	expiredState := "expired-state-123"
	auth.oidcStates[expiredState] = oidcStateEntry{
		createdAt: time.Now().Add(-15 * time.Minute), // 15 minutes ago (expired)
		nonce:     "test-nonce",
	}

	_, err := auth.validateOIDCState(expiredState)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state expired")
}

func TestAuthenticator_CleanupOIDCStates(t *testing.T) {
	auth := NewAuthenticator(config.JWTAuth{}, nil)

	// Add some states
	auth.oidcStates["fresh-state"] = oidcStateEntry{
		createdAt: time.Now(),
		nonce:     "nonce1",
	}
	auth.oidcStates["old-state"] = oidcStateEntry{
		createdAt: time.Now().Add(-15 * time.Minute),
		nonce:     "nonce2",
	}

	assert.Len(t, auth.oidcStates, 2)

	auth.cleanupOIDCStates()

	// Only fresh state should remain
	assert.Len(t, auth.oidcStates, 1)
	_, exists := auth.oidcStates["fresh-state"]
	assert.True(t, exists)
	_, exists = auth.oidcStates["old-state"]
	assert.False(t, exists)
}

func TestOIDCConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		cfg         config.OIDC
		expectError bool
		errContains string
	}{
		{
			name: "disabled - no validation",
			cfg: config.OIDC{
				Enable: false,
			},
			expectError: false,
		},
		{
			name: "enabled - missing issuer_url",
			cfg: config.OIDC{
				Enable:       true,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "https://example.com/callback",
			},
			expectError: true,
			errContains: "issuer_url",
		},
		{
			name: "enabled - missing client_id",
			cfg: config.OIDC{
				Enable:       true,
				IssuerURL:    "https://issuer.example.com",
				ClientSecret: "client-secret",
				RedirectURL:  "https://example.com/callback",
			},
			expectError: true,
			errContains: "client_id",
		},
		{
			name: "enabled - missing client_secret",
			cfg: config.OIDC{
				Enable:      true,
				IssuerURL:   "https://issuer.example.com",
				ClientID:    "client-id",
				RedirectURL: "https://example.com/callback",
			},
			expectError: true,
			errContains: "client_secret",
		},
		{
			name: "enabled - missing redirect_url",
			cfg: config.OIDC{
				Enable:       true,
				IssuerURL:    "https://issuer.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			},
			expectError: true,
			errContains: "redirect_url",
		},
		{
			name: "enabled - all required fields",
			cfg: config.OIDC{
				Enable:       true,
				IssuerURL:    "https://issuer.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				RedirectURL:  "https://example.com/callback",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOIDCGetScopes(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.OIDC
		expected []string
	}{
		{
			name:     "default scopes when empty",
			cfg:      config.OIDC{},
			expected: []string{"openid", "email", "profile"},
		},
		{
			name: "custom scopes",
			cfg: config.OIDC{
				Scopes: []string{"openid", "email"},
			},
			expected: []string{"openid", "email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetScopes()
			assert.Equal(t, tt.expected, result)
		})
	}
}
