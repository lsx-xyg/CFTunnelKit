package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
)

// fakeClient is a stub CFClient for exercising the service state machine.
type fakeClient struct {
	verifyFn func(ctx context.Context) (cloudflare.TokenInfo, error)
	listFn   func(ctx context.Context, accountID string) ([]cloudflare.Tunnel, error)
}

func (f *fakeClient) VerifyToken(ctx context.Context) (cloudflare.TokenInfo, error) {
	return f.verifyFn(ctx)
}

func (f *fakeClient) ListTunnels(ctx context.Context, accountID string) ([]cloudflare.Tunnel, error) {
	return f.listFn(ctx, accountID)
}

func newTestService(t *testing.T, c cloudflare.CFClient) (*Service, *config.Store) {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	svc := NewService(store)
	svc.newClient = func(token string) cloudflare.CFClient { return c }
	return svc, store
}

func okInfo() cloudflare.TokenInfo {
	return cloudflare.TokenInfo{
		TokenID:     "tok-1",
		AccountID:   "acct1",
		AccountName: "Acct One",
		Permissions: cloudflare.Permissions{
			TunnelEdit: cloudflare.PermissionOK,
			ZoneRead:   cloudflare.PermissionOK,
			DNSEdit:    cloudflare.PermissionUnverified,
		},
		FirstZoneID: "zone1",
		Warnings:    []string{"账户下无 Zone，DNS 权限待绑定域名后验证"},
	}
}

func TestVerifyAndSaveToken_Success(t *testing.T) {
	svc, store := newTestService(t, &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
		return okInfo(), nil
	}})

	info, err := svc.VerifyAndSaveToken(context.Background(), "  tok-abc  ")
	if err != nil {
		t.Fatalf("VerifyAndSaveToken: %v", err)
	}
	if info.AccountID != "acct1" {
		t.Errorf("AccountID = %q, want acct1", info.AccountID)
	}
	st := svc.GetState()
	if !st.Authenticated || !st.HasToken || st.Offline {
		t.Errorf("state = %+v, want authenticated", st)
	}
	if st.TokenInfo == nil || st.TokenInfo.TokenID != "tok-1" {
		t.Errorf("state.TokenInfo = %+v, want tok-1", st.TokenInfo)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIToken != "tok-abc" || cfg.AccountID != "acct1" || cfg.AccountName != "Acct One" {
		t.Errorf("cfg = %+v, want token/acct persisted", cfg)
	}
	if cfg.Permissions.TunnelEdit != "ok" || cfg.Permissions.DNSEdit != "unverified" {
		t.Errorf("cfg.Permissions = %+v, want ok/unverified mapping", cfg.Permissions)
	}
	if cfg.VerifiedAt == "" {
		t.Error("VerifiedAt should be set")
	}
}

func TestVerifyAndSaveToken_Failure_NothingSaved(t *testing.T) {
	svc, store := newTestService(t, &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
		return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 无效或已失效"}
	}})

	_, err := svc.VerifyAndSaveToken(context.Background(), "tok-bad")
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	st := svc.GetState()
	if st.Authenticated || st.HasToken {
		t.Errorf("state = %+v, want unauthenticated", st)
	}
	if _, err := os.Stat(store.Path()); err == nil {
		t.Error("config file should not exist after failed verification")
	}
}

func TestVerifyAndSaveToken_EmptyToken(t *testing.T) {
	svc, _ := newTestService(t, &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
		t.Fatal("client should not be called with an empty token")
		return cloudflare.TokenInfo{}, nil
	}})
	_, err := svc.VerifyAndSaveToken(context.Background(), "   ")
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
}

func TestRestore_NoToken_Unauthenticated(t *testing.T) {
	svc, _ := newTestService(t, &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
		t.Fatal("client should not be called without a token")
		return cloudflare.TokenInfo{}, nil
	}})
	svc.Restore(context.Background())
	st := svc.GetState()
	if st.Authenticated || st.HasToken {
		t.Errorf("state = %+v, want unauthenticated", st)
	}
}

func TestRestore_ValidToken_Authenticated(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(config.Config{APIToken: "tok-abc", AccountID: "acct1"}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store)
	svc.newClient = func(token string) cloudflare.CFClient {
		return &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
			return okInfo(), nil
		}}
	}
	svc.Restore(context.Background())
	st := svc.GetState()
	if !st.Authenticated || st.Offline {
		t.Errorf("state = %+v, want authenticated", st)
	}
}

func TestRetryVerify_AuthError_ClearsConfig(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(config.Config{APIToken: "tok-expired"}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store)
	svc.newClient = func(token string) cloudflare.CFClient {
		return &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
			return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 无效或已失效"}
		}}
	}
	svc.mu.Lock()
	svc.state = State{HasToken: true}
	svc.mu.Unlock()

	_, err := svc.RetryVerify(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if !strings.Contains(err.Error(), "已失效") {
		t.Errorf("message = %q, want to contain 已失效", err.Error())
	}
	if _, statErr := os.Stat(store.Path()); statErr == nil {
		t.Error("config should be cleared on auth failure")
	}
	st := svc.GetState()
	if st.Authenticated || st.HasToken {
		t.Errorf("state = %+v, want unauthenticated after auth failure", st)
	}
}

func TestRetryVerify_NetworkError_Offline(t *testing.T) {
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(config.Config{APIToken: "tok-abc"}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store)
	svc.newClient = func(token string) cloudflare.CFClient {
		return &fakeClient{verifyFn: func(ctx context.Context) (cloudflare.TokenInfo, error) {
			return cloudflare.TokenInfo{}, &cloudflare.APIError{Kind: cloudflare.KindNetwork, Message: "无法连接 Cloudflare API"}
		}}
	}
	svc.mu.Lock()
	svc.state = State{HasToken: true}
	svc.mu.Unlock()

	_, err := svc.RetryVerify(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
	st := svc.GetState()
	if st.Authenticated {
		t.Error("should not be authenticated while offline")
	}
	if !st.HasToken || !st.Offline {
		t.Errorf("state = %+v, want has_token + offline", st)
	}
	if !strings.Contains(st.Message, "无法连接") {
		t.Errorf("state.Message = %q, want to contain 无法连接", st.Message)
	}
}
