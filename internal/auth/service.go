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
	"github.com/lsx-xyg/CFTunnelKit/internal/ingress"
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

// ListTunnels fetches the tunnels for the persisted account (page=1,
// per_page=50, see slice 02). On an auth failure (401 / no token / no
// account) the persisted config is cleared and the state is reset, so the
// frontend can detect the session expiry via GetState and jump to the auth
// page. Permission and network errors leave the state untouched.
func (s *Service) ListTunnels(ctx context.Context) ([]cloudflare.Tunnel, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	tunnels, err := cl.ListTunnels(ctx, accountID, 1, 50)
	if err != nil {
		return nil, s.clearOnAuth(err)
	}
	return tunnels, nil
}

// GetTunnelToken fetches the run token for a tunnel (issue #5, needed to
// start cloudflared). Auth failures clear the persisted config and reset
// the state, exactly like ListTunnels.
func (s *Service) GetTunnelToken(ctx context.Context, tunnelID string) (string, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return "", err
	}
	tok, err := cl.GetTunnelToken(ctx, accountID, tunnelID)
	if err != nil {
		return "", s.clearOnAuth(err)
	}
	return tok, nil
}

// CreateTunnel creates a remotely-managed tunnel (issue #6). The name is
// validated frontend-side; 409 surfaces as "同名 Tunnel 已存在".
func (s *Service) CreateTunnel(ctx context.Context, name string) (cloudflare.Tunnel, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return cloudflare.Tunnel{}, err
	}
	t, err := cl.CreateTunnel(ctx, accountID, name)
	if err != nil {
		return cloudflare.Tunnel{}, s.clearOnAuth(err)
	}
	return t, nil
}

// DeleteTunnel deletes a tunnel (issue #6). Active-connection errors
// surface as "该 Tunnel 有活跃连接，请先停止隧道".
func (s *Service) DeleteTunnel(ctx context.Context, tunnelID string) error {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return err
	}
	return s.clearOnAuth(cl.DeleteTunnel(ctx, accountID, tunnelID))
}

// GetTunnelDetail fetches one tunnel's full record (issue #6).
func (s *Service) GetTunnelDetail(ctx context.Context, tunnelID string) (cloudflare.TunnelDetail, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return cloudflare.TunnelDetail{}, err
	}
	d, err := cl.GetTunnelDetail(ctx, accountID, tunnelID)
	if err != nil {
		return cloudflare.TunnelDetail{}, s.clearOnAuth(err)
	}
	return d, nil
}

// ListZones returns the account's zones (issue #5, hostname validation).
func (s *Service) ListZones(ctx context.Context) ([]cloudflare.Zone, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	zones, err := cl.ListZones(ctx, accountID)
	if err != nil {
		return nil, s.clearOnAuth(err)
	}
	return zones, nil
}

// GetIngressConfig returns the tunnel's ingress rules (catch-all stripped,
// issue #5).
func (s *Service) GetIngressConfig(ctx context.Context, tunnelID string) ([]cloudflare.IngressRule, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := cl.GetIngressConfig(ctx, accountID, tunnelID)
	if err != nil {
		return nil, s.clearOnAuth(err)
	}
	return rules, nil
}

// SaveIngressConfig implements the issue #5 save flow: defensive validation,
// PUT, then GET read-back compared by content+order (normalized). On
// mismatch the user's input is preserved and "配置未生效，请重试" is returned;
// on success the read-back data is returned so the frontend refreshes from
// it instead of keeping stale input.
func (s *Service) SaveIngressConfig(ctx context.Context, tunnelID string, rules []cloudflare.IngressRule) ([]cloudflare.IngressRule, error) {
	cl, accountID, err := s.authenticatedClient(ctx)
	if err != nil {
		return nil, err
	}
	if errs := ingress.Validate(rules, nil); len(errs) > 0 {
		return nil, &cloudflare.APIError{Kind: cloudflare.KindAPI, Message: errs[0].Msg}
	}
	if err := cl.PutIngressConfig(ctx, accountID, tunnelID, rules); err != nil {
		return nil, s.clearOnAuth(err)
	}
	got, err := cl.GetIngressConfig(ctx, accountID, tunnelID)
	if err != nil {
		return nil, s.clearOnAuth(err)
	}
	if !ingress.SameRules(rules, got) {
		return nil, &cloudflare.APIError{Kind: cloudflare.KindAPI, Message: "配置未生效，请重试"}
	}
	return got, nil
}

// authenticatedClient loads the persisted config and returns a client for
// the stored account. Missing/corrupt config yields an AuthError.
func (s *Service) authenticatedClient(ctx context.Context) (cloudflare.CFClient, string, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return nil, "", &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 已失效，请重新输入"}
	}
	if strings.TrimSpace(cfg.APIToken) == "" || strings.TrimSpace(cfg.AccountID) == "" {
		return nil, "", &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "账户未解析，请重新认证"}
	}
	return s.newClient(cfg.APIToken), cfg.AccountID, nil
}

// clearOnAuth clears the persisted config and resets the state when err is
// an auth failure, so the frontend can detect session expiry via GetState.
func (s *Service) clearOnAuth(err error) error {
	if cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		_ = s.store.Clear()
		s.mu.Lock()
		s.state = State{}
		s.mu.Unlock()
	}
	return err
}
