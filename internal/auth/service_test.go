package auth

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
	"github.com/lsx-xyg/CFTunnelKit/internal/config"
)

// fakeClient is a stub CFClient for exercising the service state machine.
type fakeClient struct {
	verifyFn func(ctx context.Context) (cloudflare.TokenInfo, error)
	listFn   func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error)
	tokenFn  func(ctx context.Context, accountID, tunnelID string) (string, error)
	createFn func(ctx context.Context, accountID, name string) (cloudflare.Tunnel, error)
	deleteFn func(ctx context.Context, accountID, tunnelID string) error
	detailFn func(ctx context.Context, accountID, tunnelID string) (cloudflare.TunnelDetail, error)
	ingGetFn func(ctx context.Context, accountID, tunnelID string) ([]cloudflare.IngressRule, error)
	ingPutFn func(ctx context.Context, accountID, tunnelID string, rules []cloudflare.IngressRule) error
	zonesFn  func(ctx context.Context, accountID string) ([]cloudflare.Zone, error)
	dnsListFn   func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error)
	dnsCreateFn func(ctx context.Context, zoneID, name, target string) (cloudflare.DNSRecord, error)
	dnsDeleteFn func(ctx context.Context, zoneID, recordID string) error
}

func (f *fakeClient) VerifyToken(ctx context.Context) (cloudflare.TokenInfo, error) {
	return f.verifyFn(ctx)
}

func (f *fakeClient) ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
	return f.listFn(ctx, accountID, page, perPage)
}

func (f *fakeClient) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	return f.tokenFn(ctx, accountID, tunnelID)
}

func (f *fakeClient) CreateTunnel(ctx context.Context, accountID, name string) (cloudflare.Tunnel, error) {
	return f.createFn(ctx, accountID, name)
}

func (f *fakeClient) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	return f.deleteFn(ctx, accountID, tunnelID)
}

func (f *fakeClient) GetTunnelDetail(ctx context.Context, accountID, tunnelID string) (cloudflare.TunnelDetail, error) {
	return f.detailFn(ctx, accountID, tunnelID)
}

func (f *fakeClient) GetIngressConfig(ctx context.Context, accountID, tunnelID string) ([]cloudflare.IngressRule, error) {
	return f.ingGetFn(ctx, accountID, tunnelID)
}

func (f *fakeClient) PutIngressConfig(ctx context.Context, accountID, tunnelID string, rules []cloudflare.IngressRule) error {
	return f.ingPutFn(ctx, accountID, tunnelID, rules)
}

func (f *fakeClient) ListZones(ctx context.Context, accountID string) ([]cloudflare.Zone, error) {
	return f.zonesFn(ctx, accountID)
}

func (f *fakeClient) ListDNSRecords(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
	return f.dnsListFn(ctx, zoneID)
}

func (f *fakeClient) CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (cloudflare.DNSRecord, error) {
	return f.dnsCreateFn(ctx, zoneID, name, target)
}

func (f *fakeClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	return f.dnsDeleteFn(ctx, zoneID, recordID)
}

func newTestService(t *testing.T, c cloudflare.CFClient) (*Service, *config.Store) {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	svc := NewService(store)
	svc.newClient = func(token, _ string) cloudflare.CFClient { return c }
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
	svc.newClient = func(token, _ string) cloudflare.CFClient {
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
	svc.newClient = func(token, _ string) cloudflare.CFClient {
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
	svc.newClient = func(token, _ string) cloudflare.CFClient {
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

func newListService(t *testing.T, c cloudflare.CFClient, cfg config.Config) (*Service, *config.Store) {
	t.Helper()
	store := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	svc := NewService(store)
	svc.newClient = func(token, _ string) cloudflare.CFClient { return c }
	return svc, store
}

func TestListTunnels_Success_UsesPersistedAccount(t *testing.T) {
	var gotAccount, gotPage, gotPerPage string
	svc, _ := newListService(t, &fakeClient{listFn: func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
		gotAccount, gotPage, gotPerPage = accountID, strconv.Itoa(page), strconv.Itoa(perPage)
		return []cloudflare.Tunnel{{ID: "t1", Name: "tunnel-a", Status: "healthy"}}, nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	tunnels, err := svc.ListTunnels(context.Background())
	if err != nil {
		t.Fatalf("ListTunnels: %v", err)
	}
	if len(tunnels) != 1 || tunnels[0].Name != "tunnel-a" {
		t.Fatalf("tunnels = %+v, want one tunnel-a", tunnels)
	}
	if gotAccount != "acct1" || gotPage != "1" || gotPerPage != "50" {
		t.Errorf("client called with account=%s page=%s per_page=%s, want acct1/1/50", gotAccount, gotPage, gotPerPage)
	}
}

func TestListTunnels_NoAccount_AuthError(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{listFn: func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
		t.Fatal("client should not be called without account")
		return nil, nil
	}}, config.Config{APIToken: "tok-abc"})

	_, err := svc.ListTunnels(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
}

func TestListTunnels_AuthError_ClearsConfigAndState(t *testing.T) {
	svc, store := newListService(t, &fakeClient{listFn: func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
		return nil, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 无效或已失效"}
	}}, config.Config{APIToken: "tok-expired", AccountID: "acct1"})
	svc.mu.Lock()
	svc.state = State{Authenticated: true, HasToken: true}
	svc.mu.Unlock()

	_, err := svc.ListTunnels(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr == nil {
		t.Error("config should be cleared on auth failure")
	}
	st := svc.GetState()
	if st.Authenticated || st.HasToken {
		t.Errorf("state = %+v, want reset to unauthenticated", st)
	}
}

func TestListTunnels_PermissionError_StateUntouched(t *testing.T) {
	svc, store := newListService(t, &fakeClient{listFn: func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
		return nil, &cloudflare.APIError{Kind: cloudflare.KindPermission, Message: "缺少 Tunnel:Edit 权限"}
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})
	svc.mu.Lock()
	svc.state = State{Authenticated: true, HasToken: true}
	svc.mu.Unlock()

	_, err := svc.ListTunnels(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr != nil {
		t.Error("config should NOT be cleared on permission error")
	}
	st := svc.GetState()
	if !st.Authenticated || !st.HasToken {
		t.Errorf("state = %+v, want untouched authenticated", st)
	}
}

func TestListTunnels_NetworkError_StateUntouched(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{listFn: func(ctx context.Context, accountID string, page, perPage int) ([]cloudflare.Tunnel, error) {
		return nil, &cloudflare.APIError{Kind: cloudflare.KindNetwork, Message: "无法连接 Cloudflare API"}
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})
	svc.mu.Lock()
	svc.state = State{Authenticated: true, HasToken: true}
	svc.mu.Unlock()

	_, err := svc.ListTunnels(context.Background())
	if !cloudflare.IsAPIError(err, cloudflare.KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
	st := svc.GetState()
	if !st.Authenticated || !st.HasToken {
		t.Errorf("state = %+v, want untouched authenticated", st)
	}
}

func TestGetTunnelToken_Success_UsesPersistedAccount(t *testing.T) {
	var gotAccount, gotTunnel string
	svc, _ := newListService(t, &fakeClient{tokenFn: func(ctx context.Context, accountID, tunnelID string) (string, error) {
		gotAccount, gotTunnel = accountID, tunnelID
		return "tok-run-1", nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	tok, err := svc.GetTunnelToken(context.Background(), "tun1")
	if err != nil {
		t.Fatalf("GetTunnelToken: %v", err)
	}
	if tok != "tok-run-1" {
		t.Errorf("token = %q, want tok-run-1", tok)
	}
	if gotAccount != "acct1" || gotTunnel != "tun1" {
		t.Errorf("client called with account=%s tunnel=%s, want acct1/tun1", gotAccount, gotTunnel)
	}
}

func TestGetTunnelToken_AuthError_ClearsConfigAndState(t *testing.T) {
	svc, store := newListService(t, &fakeClient{tokenFn: func(ctx context.Context, accountID, tunnelID string) (string, error) {
		return "", &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 无效或已失效"}
	}}, config.Config{APIToken: "tok-expired", AccountID: "acct1"})
	svc.mu.Lock()
	svc.state = State{Authenticated: true, HasToken: true}
	svc.mu.Unlock()

	_, err := svc.GetTunnelToken(context.Background(), "tun1")
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr == nil {
		t.Error("config should be cleared on auth failure")
	}
	st := svc.GetState()
	if st.Authenticated || st.HasToken {
		t.Errorf("state = %+v, want reset", st)
	}
}

func TestGetTunnelToken_PermissionError_StateUntouched(t *testing.T) {
	svc, store := newListService(t, &fakeClient{tokenFn: func(ctx context.Context, accountID, tunnelID string) (string, error) {
		return "", &cloudflare.APIError{Kind: cloudflare.KindPermission, Message: "缺少 Tunnel:Edit 权限"}
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})
	svc.mu.Lock()
	svc.state = State{Authenticated: true, HasToken: true}
	svc.mu.Unlock()

	_, err := svc.GetTunnelToken(context.Background(), "tun1")
	if !cloudflare.IsAPIError(err, cloudflare.KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr != nil {
		t.Error("config should NOT be cleared on permission error")
	}
	st := svc.GetState()
	if !st.Authenticated || !st.HasToken {
		t.Errorf("state = %+v, want untouched", st)
	}
}

func TestCreateTunnel_Success_PassesName(t *testing.T) {
	var gotName string
	svc, _ := newListService(t, &fakeClient{createFn: func(ctx context.Context, accountID, name string) (cloudflare.Tunnel, error) {
		gotName = name
		return cloudflare.Tunnel{ID: "t9", Name: name, Status: "inactive"}, nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	tun, err := svc.CreateTunnel(context.Background(), "my-tunnel")
	if err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if tun.Name != "my-tunnel" || tun.ID != "t9" {
		t.Errorf("tunnel = %+v, want my-tunnel/t9", tun)
	}
	if gotName != "my-tunnel" {
		t.Errorf("client got name = %q, want my-tunnel", gotName)
	}
}

func TestCreateTunnel_ConflictError_Passthrough(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{createFn: func(ctx context.Context, accountID, name string) (cloudflare.Tunnel, error) {
		return cloudflare.Tunnel{}, &cloudflare.APIError{Kind: cloudflare.KindAPI, Message: "同名 Tunnel 已存在"}
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.CreateTunnel(context.Background(), "dup")
	if !cloudflare.IsAPIError(err, cloudflare.KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "同名 Tunnel 已存在") {
		t.Errorf("message = %q, want to contain 同名 Tunnel 已存在", err.Error())
	}
}

func TestCreateTunnel_AuthError_ClearsConfig(t *testing.T) {
	svc, store := newListService(t, &fakeClient{createFn: func(ctx context.Context, accountID, name string) (cloudflare.Tunnel, error) {
		return cloudflare.Tunnel{}, &cloudflare.APIError{Kind: cloudflare.KindAuth, Message: "Token 无效或已失效"}
	}}, config.Config{APIToken: "tok-expired", AccountID: "acct1"})

	_, err := svc.CreateTunnel(context.Background(), "x")
	if !cloudflare.IsAPIError(err, cloudflare.KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr == nil {
		t.Error("config should be cleared on auth failure")
	}
}

func TestDeleteTunnel_Success(t *testing.T) {
	var deleted string
	svc, _ := newListService(t, &fakeClient{deleteFn: func(ctx context.Context, accountID, tunnelID string) error {
		deleted = tunnelID
		return nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	if err := svc.DeleteTunnel(context.Background(), "tun1"); err != nil {
		t.Fatalf("DeleteTunnel: %v", err)
	}
	if deleted != "tun1" {
		t.Errorf("deleted = %q, want tun1", deleted)
	}
}

func TestDeleteTunnel_ActiveConnections_Passthrough(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{deleteFn: func(ctx context.Context, accountID, tunnelID string) error {
		return &cloudflare.APIError{Kind: cloudflare.KindAPI, Message: "该 Tunnel 有活跃连接，请先停止隧道"}
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	err := svc.DeleteTunnel(context.Background(), "tun1")
	if !cloudflare.IsAPIError(err, cloudflare.KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "请先停止隧道") {
		t.Errorf("message = %q, want to contain 请先停止隧道", err.Error())
	}
}

func TestGetTunnelDetail_Success(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{detailFn: func(ctx context.Context, accountID, tunnelID string) (cloudflare.TunnelDetail, error) {
		return cloudflare.TunnelDetail{ID: tunnelID, Name: "tunnel-a", Status: "healthy", Connections: 2}, nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	d, err := svc.GetTunnelDetail(context.Background(), "tun1")
	if err != nil {
		t.Fatalf("GetTunnelDetail: %v", err)
	}
	if d.Connections != 2 || d.Name != "tunnel-a" {
		t.Errorf("detail = %+v, want tunnel-a/2 connections", d)
	}
}

func TestListZones_Success(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{zonesFn: func(ctx context.Context, accountID string) ([]cloudflare.Zone, error) {
		return []cloudflare.Zone{{ID: "z1", Name: "example.com"}}, nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	zones, err := svc.ListZones(context.Background())
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Errorf("zones = %+v, want example.com", zones)
	}
}

func TestGetIngressConfig_Success_StripsCatchAll(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{ingGetFn: func(ctx context.Context, accountID, tunnelID string) ([]cloudflare.IngressRule, error) {
		return []cloudflare.IngressRule{{Hostname: "nas.example.com", Service: "http://localhost:5000"}}, nil
	}}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	rules, err := svc.GetIngressConfig(context.Background(), "tun1")
	if err != nil {
		t.Fatalf("GetIngressConfig: %v", err)
	}
	if len(rules) != 1 || rules[0].Hostname != "nas.example.com" {
		t.Errorf("rules = %+v, want nas.example.com", rules)
	}
}

func TestSaveIngressConfig_Success_ReturnsReadBack(t *testing.T) {
	var putCalled, getCalled bool
	fc := &fakeClient{
		ingPutFn: func(ctx context.Context, accountID, tunnelID string, rules []cloudflare.IngressRule) error {
			putCalled = true
			if len(rules) != 1 {
				t.Errorf("put rules = %+v, want 1", rules)
			}
			return nil
		},
		ingGetFn: func(ctx context.Context, accountID, tunnelID string) ([]cloudflare.IngressRule, error) {
			getCalled = true
			return []cloudflare.IngressRule{{Hostname: "nas.example.com", Service: "http://localhost:5000"}}, nil
		},
	}
	svc, _ := newListService(t, fc, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	got, err := svc.SaveIngressConfig(context.Background(), "tun1", []cloudflare.IngressRule{
		{Hostname: "nas.example.com", Service: "http://localhost:5000"},
	})
	if err != nil {
		t.Fatalf("SaveIngressConfig: %v", err)
	}
	if !putCalled || !getCalled {
		t.Errorf("put=%v get=%v, want both called", putCalled, getCalled)
	}
	if len(got) != 1 || got[0].Hostname != "nas.example.com" {
		t.Errorf("got = %+v, want read-back data", got)
	}
}

func TestSaveIngressConfig_ReadBackMismatch_ErrorPreservesInput(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		ingPutFn: func(ctx context.Context, accountID, tunnelID string, rules []cloudflare.IngressRule) error {
			return nil
		},
		ingGetFn: func(ctx context.Context, accountID, tunnelID string) ([]cloudflare.IngressRule, error) {
			// Cloudflare dropped one rule → mismatch
			return nil, nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.SaveIngressConfig(context.Background(), "tun1", []cloudflare.IngressRule{
		{Hostname: "nas.example.com", Service: "http://localhost:5000"},
	})
	if !cloudflare.IsAPIError(err, cloudflare.KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "配置未生效，请重试") {
		t.Errorf("message = %q, want 配置未生效", err.Error())
	}
}

func TestSaveIngressConfig_EmptyRules_Rejected(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.SaveIngressConfig(context.Background(), "tun1", nil)
	if !cloudflare.IsAPIError(err, cloudflare.KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "至少需要一条规则") {
		t.Errorf("message = %q, want 至少需要一条规则", err.Error())
	}
}

func TestSaveIngressConfig_PutPermissionError_Passthrough(t *testing.T) {
	svc, store := newListService(t, &fakeClient{
		ingPutFn: func(ctx context.Context, accountID, tunnelID string, rules []cloudflare.IngressRule) error {
			return &cloudflare.APIError{Kind: cloudflare.KindPermission, Message: "缺少 Tunnel:Edit 权限"}
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.SaveIngressConfig(context.Background(), "tun1", []cloudflare.IngressRule{
		{Hostname: "nas.example.com", Service: "http://localhost:5000"},
	})
	if !cloudflare.IsAPIError(err, cloudflare.KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if _, statErr := os.Stat(store.Path()); statErr != nil {
		t.Error("config should NOT be cleared on permission error")
	}
}

func TestEnsureCNAME_Created(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return nil, nil
		},
		dnsCreateFn: func(ctx context.Context, zoneID, name, target string) (cloudflare.DNSRecord, error) {
			return cloudflare.DNSRecord{ID: "r9", Type: "CNAME", Name: name, Content: target}, nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	res, err := svc.EnsureCNAME(context.Background(), "z1", "nas", "tun1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME: %v", err)
	}
	if !res.Created || res.RecordID != "r9" {
		t.Errorf("res = %+v, want created r9", res)
	}
}

func TestEnsureCNAME_Idempotent(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return []cloudflare.DNSRecord{{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"}}, nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	res, err := svc.EnsureCNAME(context.Background(), "z1", "nas", "tun1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME: %v", err)
	}
	if res.Created || res.RecordID != "r1" {
		t.Errorf("res = %+v, want idempotent r1", res)
	}
}

func TestEnsureCNAME_Taken_SurfacesMessage(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return []cloudflare.DNSRecord{{ID: "r1", Type: "A", Name: "nas", Content: "1.2.3.4"}}, nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.EnsureCNAME(context.Background(), "z1", "nas", "tun1.cfargotunnel.com")
	if !cloudflare.IsAPIError(err, cloudflare.KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "已被占用，请手动处理") {
		t.Errorf("message = %q, want 已被占用", err.Error())
	}
}

func TestDeleteDNSByName_Deleted(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return []cloudflare.DNSRecord{{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"}}, nil
		},
		dnsDeleteFn: func(ctx context.Context, zoneID, recordID string) error {
			return nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	deleted, err := svc.DeleteDNSByName(context.Background(), "z1", "nas")
	if err != nil {
		t.Fatalf("DeleteDNSByName: %v", err)
	}
	if !deleted {
		t.Error("deleted = false, want true")
	}
}

func TestDeleteDNSByName_Failure_SurfacesError(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return []cloudflare.DNSRecord{{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"}}, nil
		},
		dnsDeleteFn: func(ctx context.Context, zoneID, recordID string) error {
			return &cloudflare.APIError{Kind: cloudflare.KindNetwork, Message: "无法连接 Cloudflare API"}
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	_, err := svc.DeleteDNSByName(context.Background(), "z1", "nas")
	if !cloudflare.IsAPIError(err, cloudflare.KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

func TestListDNSRecords_OK(t *testing.T) {
	svc, _ := newListService(t, &fakeClient{
		dnsListFn: func(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
			return []cloudflare.DNSRecord{{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"}}, nil
		},
	}, config.Config{APIToken: "tok-abc", AccountID: "acct1"})

	records, err := svc.ListDNSRecords(context.Background(), "z1")
	if err != nil {
		t.Fatalf("ListDNSRecords: %v", err)
	}
	if len(records) != 1 || records[0].Name != "nas" {
		t.Errorf("records = %+v, want nas", records)
	}
}
