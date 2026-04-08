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
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	"github.com/cloudbase/garm/config"
	"github.com/cloudbase/garm/database/common"
)

// OIDCAuthenticator handles OIDC authentication
type OIDCAuthenticator struct {
	cfg      config.OIDC
	store    common.Store
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth2   oauth2.Config

	// State management for OIDC flow
	stateMu sync.RWMutex
	states  map[string]stateEntry
}

type stateEntry struct {
	createdAt time.Time
	nonce     string
}

// NewOIDCAuthenticator creates a new OIDC authenticator
func NewOIDCAuthenticator(ctx context.Context, cfg config.OIDC, store common.Store) (*OIDCAuthenticator, error) {
	if !cfg.Enable {
		return nil, nil
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.GetScopes(),
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCAuthenticator{
		cfg:      cfg,
		store:    store,
		provider: provider,
		verifier: verifier,
		oauth2:   oauth2Config,
		states:   make(map[string]stateEntry),
	}, nil
}

// generateState creates a cryptographically secure random state
func (o *OIDCAuthenticator) generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateNonce creates a cryptographically secure random nonce
func (o *OIDCAuthenticator) generateNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the OIDC authorization URL
func (o *OIDCAuthenticator) GetAuthURL() (string, string, error) {
	state, err := o.generateState()
	if err != nil {
		return "", "", err
	}

	nonce, err := o.generateNonce()
	if err != nil {
		return "", "", err
	}

	// Store state with expiration
	o.stateMu.Lock()
	o.states[state] = stateEntry{
		createdAt: time.Now(),
		nonce:     nonce,
	}
	o.stateMu.Unlock()

	// Clean up old states
	go o.cleanupStates()

	url := o.oauth2.AuthCodeURL(state, oidc.Nonce(nonce))
	return url, state, nil
}

// cleanupStates removes expired states (older than 10 minutes)
func (o *OIDCAuthenticator) cleanupStates() {
	o.stateMu.Lock()
	defer o.stateMu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for state, entry := range o.states {
		if entry.createdAt.Before(cutoff) {
			delete(o.states, state)
		}
	}
}

// ValidateState checks if the state is valid and returns the nonce
func (o *OIDCAuthenticator) ValidateState(state string) (string, error) {
	o.stateMu.Lock()
	defer o.stateMu.Unlock()

	entry, ok := o.states[state]
	if !ok {
		return "", runnerErrors.NewBadRequestError("invalid state")
	}

	// Check if state is expired (10 minutes)
	if time.Since(entry.createdAt) > 10*time.Minute {
		delete(o.states, state)
		return "", runnerErrors.NewBadRequestError("state expired")
	}

	// Delete state after use (one-time use)
	delete(o.states, state)
	return entry.nonce, nil
}
