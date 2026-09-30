// Package auth owns the authentication state machine: verify-and-save on
// first login, restore on startup, and retry when offline. It sits between
// the Wails bindings and the cloudflare/config packages.
package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
)

// State is the serializable authentication state the frontend renders.
type State struct {
	Authenticated bool                  `json:"authenticated"`
	HasToken      bool                  `json:"has_token"`
	Offline       bool                  `json:"offline"`
	Message       string                `json:"message"`
	TokenInfo     *cloudflare.TokenInfo `json:"token_info"`
}

// Service owns the authentication state machine.
type Service struct {
	store *config.Store
	// newClient is the client factory; tests replace it with a fake.
	newClient func(token string) cloudflare.CFClient

	mu    sync.Mutex
	state State
}

// NewService builds a Service bound to the given config store.
func NewService(store *config.Store) *Service {
	return &Service{
		store: store,
		newClient: func(token string) cloudflare.CFClient {
			return cloudflare.New(cloudflare.Options{Token: token})
		},
		state: State{},
	}
}

// GetState returns a copy of the current authentication state.
func (s *Service) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// VerifyAndSaveToken validates a user-supplied token, persists it to the
// config store and flips the state to authenticated. On failure nothing is
// saved and the state is left untouched.
func (s *Service) VerifyAndSaveToken(ctx context.Context, token string) (cloudflare.TokenInfo, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 不能为空"}
	}

	info, err := s.newClient(token).VerifyToken(ctx)
	if err != nil {
		return cloudflare.TokenInfo{}, err
	}

	cfg := config.Config{
		Version:     config.ConfigVersion,
		APIToken:    token,
		AccountID:   info.AccountID,
		AccountName: info.AccountName,
		Permissions: config.Permissions{
			TunnelEdit: string(info.Permissions.TunnelEdit),
			ZoneRead:   string(info.Permissions.ZoneRead),
			DNSEdit:    string(info.Permissions.DNSEdit),
		},
		FirstZoneID: info.FirstZoneID,
		VerifiedAt:  time.Now().Format(time.RFC3339),
	}
	if err := s.store.Save(cfg); err != nil {
		return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindNetwork, Message: "配置保存失败", Cause: err}
	}

	s.mu.Lock()
	s.state = State{Authenticated: true, HasToken: true, TokenInfo: &info}
	s.mu.Unlock()
	return info, nil
}

// LoadPersisted synchronously marks the state as "has token" when a token is
// persisted, without touching the network. App.startup calls it so the
// frontend can deterministically trigger RetryVerify after boot (no race
// between a background restore and the first GetAuthState).
func (s *Service) LoadPersisted() {
	cfg, err := s.store.Load()
	if err != nil {
		// Corrupt config → treat as unauthenticated, do not crash.
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
		return
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.state = State{HasToken: true}
	s.mu.Unlock()
}

// Restore recovers the authentication state at startup using the persisted
// token. It never blocks: callers should run it in a goroutine.
func (s *Service) Restore(ctx context.Context) {
	cfg, err := s.store.Load()
	if err != nil {
		// Corrupt config → treat as unauthenticated, do not crash.
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
		return
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.state = State{HasToken: true}
	s.mu.Unlock()
	_, _ = s.RetryVerify(ctx)
}

// RetryVerify re-verifies the persisted token. It is used by the startup
// restore path and by the offline "重试" button. On auth failure it clears
// the config; on network failure it flips the state to offline.
func (s *Service) RetryVerify(ctx context.Context) (cloudflare.TokenInfo, error) {
	cfg, err := s.store.Load()
	if err != nil || strings.TrimSpace(cfg.APIToken) == "" {
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
		return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 已失效，请重新输入"}
	}

	info, err := s.newClient(cfg.APIToken).VerifyToken(ctx)
	if err != nil {
		switch {
		case cloudflare.IsAPIError(err, cloudflare.KindAuth):
			_ = s.store.Clear()
			s.mu.Lock()
			s.state = State{}
			s.mu.Unlock()
			return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 已失效，请重新输入"}
		default:
			// Network or other API failure → stay logged-in-ish but offline.
			msg := err.Error()
			var ae *cloudflare.APIError
			if errors.As(err, &ae) {
				msg = ae.Message
			}
			s.mu.Lock()
			s.state = State{HasToken: true, Offline: true, Message: msg}
			s.mu.Unlock()
			return cloudflare.TokenInfo{}, err
		}
	}

	s.mu.Lock()
	s.state = State{Authenticated: true, HasToken: true, TokenInfo: &info}
	s.mu.Unlock()
	return info, nil
}
