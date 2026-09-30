package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "test-token"

// newTestServer builds an httptest server that asserts the Authorization
// header on every request and routes by path.
func newTestServer(t *testing.T, routes map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for pattern, h := range routes {
		mux.HandleFunc(pattern, h)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("%s: Authorization = %q, want Bearer %s", r.URL.Path, got, testToken)
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func cfOK(result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"errors":  []interface{}{},
		"result":  result,
	}
}

func cfErr(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"errors":  []map[string]interface{}{{"code": statusCode * 100, "message": "denied"}},
		})
	}
}

func okJSON(result interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(cfOK(result))
	}
}

// routeCount records how many times a path was requested.
type routeCount struct {
	m map[string]int
}

func (rc *routeCount) wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc.m[r.URL.Path]++
		h(w, r)
	}
}

func TestVerifyToken_ValidToken_AllPermissionsOK(t *testing.T) {
	rc := &routeCount{m: map[string]int{}}
	verify := okJSON(map[string]interface{}{"id": "tok-1", "status": "active"})
	accounts := okJSON([]map[string]interface{}{{"id": "acct1", "name": "Acct One"}})
	tunnels := okJSON([]interface{}{})
	zones := okJSON([]map[string]interface{}{{"id": "zone1", "name": "example.com"}})
	dns := okJSON([]interface{}{})

	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify":       rc.wrap(verify),
		"GET /accounts":                 rc.wrap(accounts),
		"GET /accounts/acct1/cfd_tunnel": rc.wrap(tunnels),
		"GET /zones":                    rc.wrap(zones),
		"GET /zones/zone1/dns_records":  rc.wrap(dns),
	})

	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	info, err := c.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken: unexpected error: %v", err)
	}
	if info.TokenID != "tok-1" {
		t.Errorf("TokenID = %q, want tok-1", info.TokenID)
	}
	if info.AccountID != "acct1" || info.AccountName != "Acct One" {
		t.Errorf("account = %q/%q, want acct1/Acct One", info.AccountID, info.AccountName)
	}
	if info.Permissions.TunnelEdit != PermissionOK || info.Permissions.ZoneRead != PermissionOK || info.Permissions.DNSEdit != PermissionOK {
		t.Errorf("permissions = %+v, want all ok", info.Permissions)
	}
	if info.FirstZoneID != "zone1" {
		t.Errorf("FirstZoneID = %q, want zone1", info.FirstZoneID)
	}
	if len(info.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", info.Warnings)
	}
	for _, p := range []string{"/user/tokens/verify", "/accounts", "/accounts/acct1/cfd_tunnel", "/zones", "/zones/zone1/dns_records"} {
		if rc.m[p] != 1 {
			t.Errorf("endpoint %s called %d times, want 1", p, rc.m[p])
		}
	}
}

func TestVerifyToken_InvalidToken_401(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify": cfErr(http.StatusUnauthorized),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.VerifyToken(context.Background())
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if !strings.Contains(err.Error(), "Token 无效或已失效") {
		t.Errorf("message = %q, want to contain 无效或已失效", err.Error())
	}
}

func TestVerifyToken_InvalidToken_InactiveStatus(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify": okJSON(map[string]interface{}{"id": "tok-1", "status": "inactive"}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.VerifyToken(context.Background())
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
}

func TestVerifyToken_MissingPermissions(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify":       okJSON(map[string]interface{}{"id": "tok-1", "status": "active"}),
		"GET /accounts":                 okJSON([]map[string]interface{}{{"id": "acct1", "name": "Acct One"}}),
		"GET /accounts/acct1/cfd_tunnel": cfErr(http.StatusForbidden),
		"GET /zones":                    cfErr(http.StatusForbidden),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	info, err := c.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken: unexpected error: %v", err)
	}
	if info.Permissions.TunnelEdit != PermissionMissing {
		t.Errorf("TunnelEdit = %q, want missing", info.Permissions.TunnelEdit)
	}
	if info.Permissions.ZoneRead != PermissionMissing || info.Permissions.DNSEdit != PermissionMissing {
		t.Errorf("ZoneRead/DNSEdit = %q/%q, want missing/missing", info.Permissions.ZoneRead, info.Permissions.DNSEdit)
	}
}

func TestVerifyToken_NoZone_DNSEditUnverified(t *testing.T) {
	rc := &routeCount{m: map[string]int{}}
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify":       rc.wrap(okJSON(map[string]interface{}{"id": "tok-1", "status": "active"})),
		"GET /accounts":                 rc.wrap(okJSON([]map[string]interface{}{{"id": "acct1", "name": "Acct One"}})),
		"GET /accounts/acct1/cfd_tunnel": rc.wrap(okJSON([]interface{}{})),
		"GET /zones":                    rc.wrap(okJSON([]interface{}{})),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	info, err := c.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken: unexpected error: %v", err)
	}
	if info.Permissions.ZoneRead != PermissionOK {
		t.Errorf("ZoneRead = %q, want ok", info.Permissions.ZoneRead)
	}
	if info.Permissions.DNSEdit != PermissionUnverified {
		t.Errorf("DNSEdit = %q, want unverified", info.Permissions.DNSEdit)
	}
	if info.FirstZoneID != "" {
		t.Errorf("FirstZoneID = %q, want empty", info.FirstZoneID)
	}
	found := false
	for _, w := range info.Warnings {
		if strings.Contains(w, "无 Zone") {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want a no-zone warning", info.Warnings)
	}
	if _, hit := rc.m["/zones/zone1/dns_records"]; hit {
		t.Error("dns_records probe should not be called when there is no zone")
	}
}

func TestVerifyToken_AccountFallback_ToMemberships(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify": okJSON(map[string]interface{}{"id": "tok-1", "status": "active"}),
		"GET /accounts":           cfErr(http.StatusForbidden),
		"GET /user/memberships": okJSON([]map[string]interface{}{
			{"account": map[string]interface{}{"id": "acct9", "name": "Fallback Acct"}},
		}),
		"GET /accounts/acct9/cfd_tunnel": okJSON([]interface{}{}),
		"GET /zones":                    okJSON([]interface{}{}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	info, err := c.VerifyToken(context.Background())
	if err != nil {
		t.Fatalf("VerifyToken: unexpected error: %v", err)
	}
	if info.AccountID != "acct9" || info.AccountName != "Fallback Acct" {
		t.Errorf("account = %q/%q, want acct9/Fallback Acct", info.AccountID, info.AccountName)
	}
	found := false
	for _, w := range info.Warnings {
		if strings.Contains(w, "/user/memberships") {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want a memberships-fallback warning", info.Warnings)
	}
}

func TestVerifyToken_AccountResolutionBothFail(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify": okJSON(map[string]interface{}{"id": "tok-1", "status": "active"}),
		"GET /accounts":           cfErr(http.StatusForbidden),
		"GET /user/memberships":   cfErr(http.StatusForbidden),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.VerifyToken(context.Background())
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if !strings.Contains(err.Error(), "无法解析账户") {
		t.Errorf("message = %q, want to contain 无法解析账户", err.Error())
	}
}

func TestVerifyToken_ServerError500_NetworkError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /user/tokens/verify": cfErr(http.StatusInternalServerError),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.VerifyToken(context.Background())
	if !IsAPIError(err, KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

func TestVerifyToken_ConnectionRefused_NetworkError(t *testing.T) {
	// A server that is closed immediately: dial fails → NetworkError.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := New(Options{BaseURL: url, Token: testToken}).(*client)
	_, err := c.VerifyToken(context.Background())
	if !IsAPIError(err, KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

func TestListTunnels_EmptyAccountID_AuthError(t *testing.T) {
	c := New(Options{Token: testToken}).(*client)
	_, err := c.ListTunnels(context.Background(), "", 1, 50)
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if !strings.Contains(err.Error(), "账户未解析") {
		t.Errorf("message = %q, want to contain 账户未解析", err.Error())
	}
}

// listTunnelsHandler asserts the page/per_page query parameters and returns
// a fixed tunnel list.
func listTunnelsHandler(t *testing.T, wantPage, wantPerPage string, result interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("page"); got != wantPage {
			t.Errorf("page = %q, want %q", got, wantPage)
		}
		if got := q.Get("per_page"); got != wantPerPage {
			t.Errorf("per_page = %q, want %q", got, wantPerPage)
		}
		_ = json.NewEncoder(w).Encode(cfOK(result))
	}
}

func TestListTunnels_OK_PinsPage1PerPage50(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": listTunnelsHandler(t, "1", "50", []map[string]interface{}{
			{"id": "t1", "name": "tunnel-a", "status": "healthy", "created_at": "2026-01-01T00:00:00Z"},
		}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	tunnels, err := c.ListTunnels(context.Background(), "acct1", 1, 50)
	if err != nil {
		t.Fatalf("ListTunnels: unexpected error: %v", err)
	}
	if len(tunnels) != 1 {
		t.Fatalf("len = %d, want 1", len(tunnels))
	}
	if tunnels[0].ID != "t1" || tunnels[0].Name != "tunnel-a" || tunnels[0].Status != "healthy" {
		t.Errorf("tunnel = %+v, want t1/tunnel-a/healthy", tunnels[0])
	}
	if tunnels[0].CreatedAt.IsZero() {
		t.Error("CreatedAt should be parsed")
	}
}

func TestListTunnels_PassesThroughPageAndPerPage(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": listTunnelsHandler(t, "2", "25", []interface{}{}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	if _, err := c.ListTunnels(context.Background(), "acct1", 2, 25); err != nil {
		t.Fatalf("ListTunnels: unexpected error: %v", err)
	}
}

func TestListTunnels_ZeroArgs_DefaultsToPage1PerPage50(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": listTunnelsHandler(t, "1", "50", []interface{}{}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	if _, err := c.ListTunnels(context.Background(), "acct1", 0, 0); err != nil {
		t.Fatalf("ListTunnels: unexpected error: %v", err)
	}
}

func TestListTunnels_Unauthorized_AuthError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": cfErr(http.StatusUnauthorized),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.ListTunnels(context.Background(), "acct1", 1, 50)
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
	if !strings.Contains(err.Error(), "Token 无效或已失效") {
		t.Errorf("message = %q, want to contain 无效或已失效", err.Error())
	}
}

func TestListTunnels_Forbidden_PermissionError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": cfErr(http.StatusForbidden),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.ListTunnels(context.Background(), "acct1", 1, 50)
	if !IsAPIError(err, KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if !strings.Contains(err.Error(), "Tunnel:Edit") {
		t.Errorf("message = %q, want to contain Tunnel:Edit", err.Error())
	}
}

func TestListTunnels_ServerError500_NetworkError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel": cfErr(http.StatusInternalServerError),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.ListTunnels(context.Background(), "acct1", 1, 50)
	if !IsAPIError(err, KindNetwork) {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
}

func TestGetTunnelToken_OK(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1/token": okJSON(map[string]interface{}{
			"token": "tok-secret-123",
		}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	tok, err := c.GetTunnelToken(context.Background(), "acct1", "tun1")
	if err != nil {
		t.Fatalf("GetTunnelToken: %v", err)
	}
	if tok != "tok-secret-123" {
		t.Errorf("token = %q, want tok-secret-123", tok)
	}
}

func TestGetTunnelToken_EmptyResult_TokenEmptyError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1/token": okJSON(map[string]interface{}{}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.GetTunnelToken(context.Background(), "acct1", "tun1")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "运行 Token 为空") {
		t.Errorf("message = %q, want to contain 运行 Token 为空", err.Error())
	}
}

func TestGetTunnelToken_NotFound_APIError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/nope/token": cfErr(http.StatusNotFound),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.GetTunnelToken(context.Background(), "acct1", "nope")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "资源不存在") {
		t.Errorf("message = %q, want to contain 资源不存在", err.Error())
	}
}

func TestGetTunnelToken_Forbidden_PermissionError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1/token": cfErr(http.StatusForbidden),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.GetTunnelToken(context.Background(), "acct1", "tun1")
	if !IsAPIError(err, KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if !strings.Contains(err.Error(), "Tunnel:Edit") {
		t.Errorf("message = %q, want to contain Tunnel:Edit", err.Error())
	}
}

func TestGetTunnelToken_EmptyArgs_AuthError(t *testing.T) {
	c := New(Options{Token: testToken}).(*client)
	_, err := c.GetTunnelToken(context.Background(), "", "tun1")
	if !IsAPIError(err, KindAuth) {
		t.Fatalf("error = %v, want KindAuth", err)
	}
}

// errWithCode returns a handler responding status with the given CF error code.
func errWithCode(statusCode, code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"errors":  []map[string]interface{}{{"code": code, "message": "denied"}},
		})
	}
}

func TestCreateTunnel_OK_SendsConfigSrc(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /accounts/acct1/cfd_tunnel": func(w http.ResponseWriter, r *http.Request) {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["name"] != "my-tunnel" {
				t.Errorf("name = %q, want my-tunnel", body["name"])
			}
			if body["config_src"] != "cloudflare" {
				t.Errorf("config_src = %q, want cloudflare", body["config_src"])
			}
			_ = json.NewEncoder(w).Encode(cfOK(map[string]interface{}{
				"id": "t9", "name": "my-tunnel", "status": "inactive", "created_at": "2026-01-02T00:00:00Z",
			}))
		},
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	tun, err := c.CreateTunnel(context.Background(), "acct1", "my-tunnel")
	if err != nil {
		t.Fatalf("CreateTunnel: %v", err)
	}
	if tun.ID != "t9" || tun.Name != "my-tunnel" {
		t.Errorf("tunnel = %+v, want t9/my-tunnel", tun)
	}
}

func TestCreateTunnel_Conflict_DuplicateName(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /accounts/acct1/cfd_tunnel": errWithCode(http.StatusConflict, 40900),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.CreateTunnel(context.Background(), "acct1", "dup")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "同名 Tunnel 已存在") {
		t.Errorf("message = %q, want to contain 同名 Tunnel 已存在", err.Error())
	}
}

func TestCreateTunnel_Forbidden_PermissionError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"POST /accounts/acct1/cfd_tunnel": cfErr(http.StatusForbidden),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.CreateTunnel(context.Background(), "acct1", "x")
	if !IsAPIError(err, KindPermission) {
		t.Fatalf("error = %v, want KindPermission", err)
	}
	if !strings.Contains(err.Error(), "Tunnel:Edit") {
		t.Errorf("message = %q, want to contain Tunnel:Edit", err.Error())
	}
}

func TestDeleteTunnel_OK(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /accounts/acct1/cfd_tunnel/tun1": okJSON(map[string]interface{}{"id": "tun1"}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	if err := c.DeleteTunnel(context.Background(), "acct1", "tun1"); err != nil {
		t.Fatalf("DeleteTunnel: %v", err)
	}
}

func TestDeleteTunnel_ActiveConnections_Code1003(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /accounts/acct1/cfd_tunnel/tun1": errWithCode(http.StatusBadRequest, 1003),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	err := c.DeleteTunnel(context.Background(), "acct1", "tun1")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
	if !strings.Contains(err.Error(), "该 Tunnel 有活跃连接，请先停止隧道") {
		t.Errorf("message = %q, want to contain 请先停止隧道", err.Error())
	}
}

func TestDeleteTunnel_NotFound_APIError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /accounts/acct1/cfd_tunnel/nope": cfErr(http.StatusNotFound),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	err := c.DeleteTunnel(context.Background(), "acct1", "nope")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
}

func TestGetTunnelDetail_OK_CountsConnections(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1": okJSON(map[string]interface{}{
			"id": "tun1", "name": "tunnel-a", "status": "healthy",
			"created_at": "2026-01-01T00:00:00Z",
			"connections": []map[string]interface{}{
				{"colostate": "connected"},
				{"colostate": "connected"},
			},
		}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	d, err := c.GetTunnelDetail(context.Background(), "acct1", "tun1")
	if err != nil {
		t.Fatalf("GetTunnelDetail: %v", err)
	}
	if d.ID != "tun1" || d.Name != "tunnel-a" || d.Status != "healthy" {
		t.Errorf("detail = %+v, want tun1/tunnel-a/healthy", d)
	}
	if d.Connections != 2 {
		t.Errorf("connections = %d, want 2", d.Connections)
	}
	if d.CreatedAt.IsZero() {
		t.Error("CreatedAt should be parsed")
	}
}

func TestGetTunnelDetail_NotFound_APIError(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/nope": cfErr(http.StatusNotFound),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	_, err := c.GetTunnelDetail(context.Background(), "acct1", "nope")
	if !IsAPIError(err, KindAPI) {
		t.Fatalf("error = %v, want KindAPI", err)
	}
}

func TestGetIngressConfig_NullConfig_EmptyList(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1/configurations": okJSON(map[string]interface{}{
			"config": nil,
		}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	rules, err := c.GetIngressConfig(context.Background(), "acct1", "tun1")
	if err != nil {
		t.Fatalf("GetIngressConfig: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("rules = %+v, want empty for null config", rules)
	}
}

func TestGetIngressConfig_StripsCatchAll(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /accounts/acct1/cfd_tunnel/tun1/configurations": okJSON(map[string]interface{}{
			"config": map[string]interface{}{
				"ingress": []map[string]interface{}{
					{"hostname": "nas.example.com", "service": "http://localhost:5000"},
					{"hostname": "*.example.com", "service": "http://localhost:5001"},
					{"service": "http_status:404"},
				},
			},
		}),
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	rules, err := c.GetIngressConfig(context.Background(), "acct1", "tun1")
	if err != nil {
		t.Fatalf("GetIngressConfig: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules = %+v, want 2 (catch-all stripped)", rules)
	}
	if rules[1].Hostname != "*.example.com" {
		t.Errorf("rules[1] = %+v, want *.example.com", rules[1])
	}
}

func TestPutIngressConfig_BodyNestedWithCatchAll(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /accounts/acct1/cfd_tunnel/tun1/configurations": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %s, want PUT", r.Method)
			}
			var body struct {
				Config struct {
					Ingress []IngressRule `json:"ingress"`
				} `json:"config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(body.Config.Ingress) != 2 {
				t.Fatalf("ingress = %+v, want 2 rules (1 + catch-all)", body.Config.Ingress)
			}
			last := body.Config.Ingress[1]
			if last.Hostname != "" || last.Service != "http_status:404" {
				t.Errorf("catch-all = %+v, want {service: http_status:404}", last)
			}
			_ = json.NewEncoder(w).Encode(cfOK(map[string]interface{}{"config": body.Config}))
		},
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	if err := c.PutIngressConfig(context.Background(), "acct1", "tun1", []IngressRule{
		{Hostname: "nas.example.com", Service: "http://localhost:5000"},
	}); err != nil {
		t.Fatalf("PutIngressConfig: %v", err)
	}
}

func TestListZones_OK(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"GET /zones": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("account.id"); got != "acct1" {
				t.Errorf("account.id = %q, want acct1", got)
			}
			_ = json.NewEncoder(w).Encode(cfOK([]map[string]interface{}{
				{"id": "z1", "name": "example.com"},
				{"id": "z2", "name": "b.example.com"},
			}))
		},
	})
	c := New(Options{BaseURL: srv.URL, Token: testToken}).(*client)
	zones, err := c.ListZones(context.Background(), "acct1")
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}
	if len(zones) != 2 || zones[1].Name != "b.example.com" {
		t.Errorf("zones = %+v, want 2 with b.example.com", zones)
	}
}
