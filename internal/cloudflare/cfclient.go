package cloudflare

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrorKind classifies an APIError so the frontend can render the right
// message and the httptest assertions can check the mapping table from the
// issue.
type ErrorKind string

const (
	// KindNetwork covers unreachable API, 5xx, timeout and JSON parse failure.
	KindNetwork ErrorKind = "network"
	// KindAuth covers invalid/expired tokens and account resolution failure.
	KindAuth ErrorKind = "auth"
	// KindPermission covers 403 responses on probe/operation endpoints.
	KindPermission ErrorKind = "permission"
	// KindAPI covers other API-level failures (e.g. 404).
	KindAPI ErrorKind = "api"
)

// APIError is the user-facing error returned by CFClient operations.
// Message is intended to be shown to the user as-is.
type APIError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.Cause }

// IsAPIError reports whether err is an *APIError of the given kind.
func IsAPIError(err error, kind ErrorKind) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Kind == kind
}

const (
	defaultBaseURL = "https://api.cloudflare.com/client/v4"
	defaultTimeout = 10 * time.Second
	defaultUA      = "CFTunnelKit/0.1"
)

// Options configures a CFClient.
type Options struct {
	BaseURL   string        // defaults to https://api.cloudflare.com/client/v4
	Token     string        // Cloudflare API token
	Timeout   time.Duration // defaults to 10s
	UserAgent string        // defaults to CFTunnelKit/0.1
}

type client struct {
	baseURL string
	token   string
	hc      *http.Client
	ua      string
}

// New builds the HTTP implementation of CFClient.
func New(opts Options) CFClient {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   strings.TrimSpace(opts.Token),
		hc:      &http.Client{Timeout: timeout},
		ua:      ua,
	}
}

// cfError mirrors a single entry of the Cloudflare errors array.
type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// getJSON performs a GET against the API and decodes `result` into out.
// perm, when non-empty, is used to build the 403 permission message.
// If out is nil, the result payload is discarded.
func (c *client) getJSON(ctx context.Context, path string, out interface{}, perm string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API", Cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.ua)
	req.Header.Set("Accept", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API", Cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API", Cause: err}
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var wrapper struct {
			Success bool      `json:"success"`
			Errors  []cfError `json:"errors"`
			Result  json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API", Cause: fmt.Errorf("解析响应失败: %w", err)}
		}
		if !wrapper.Success {
			msg := "Cloudflare API 请求失败"
			if len(wrapper.Errors) > 0 {
				msg = wrapper.Errors[0].Message
			}
			return &APIError{Kind: KindNetwork, Message: msg}
		}
		if out != nil {
			if err := json.Unmarshal(wrapper.Result, out); err != nil {
				return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API", Cause: fmt.Errorf("解析响应数据失败: %w", err)}
			}
		}
		return nil
	case http.StatusUnauthorized:
		return &APIError{Kind: KindAuth, Message: "Token 无效或已失效"}
	case http.StatusForbidden:
		if perm != "" {
			return &APIError{Kind: KindPermission, Message: "缺少 " + perm + " 权限"}
		}
		return &APIError{Kind: KindPermission, Message: "缺少 Cloudflare API 权限"}
	case http.StatusNotFound:
		return &APIError{Kind: KindAPI, Message: "Cloudflare API 资源不存在"}
	default:
		return &APIError{Kind: KindNetwork, Message: "无法连接 Cloudflare API"}
	}
}

// probe runs a read-only permission probe. 403 maps to PermissionMissing,
// 401 maps to AuthError, everything else maps to the underlying error.
func (c *client) probe(ctx context.Context, path, perm string) (PermissionStatus, error) {
	err := c.getJSON(ctx, path, nil, perm)
	if err == nil {
		return PermissionOK, nil
	}
	if IsAPIError(err, KindPermission) {
		return PermissionMissing, nil
	}
	return "", err
}

// VerifyToken implements CFClient.VerifyToken.
func (c *client) VerifyToken(ctx context.Context) (TokenInfo, error) {
	if c.token == "" {
		return TokenInfo{}, &APIError{Kind: KindAuth, Message: "Token 不能为空"}
	}

	// 1. Token validation.
	var vr struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := c.getJSON(ctx, "/user/tokens/verify", &vr, ""); err != nil {
		return TokenInfo{}, err
	}
	if vr.Status != "active" {
		return TokenInfo{}, &APIError{Kind: KindAuth, Message: "Token 无效或已失效"}
	}
	info := TokenInfo{TokenID: vr.ID}

	// 2. Account resolution: /accounts, falling back to /user/memberships
	// on 403 or empty result.
	acct, fallbackUsed, err := c.resolveAccount(ctx)
	if err != nil {
		return TokenInfo{}, err
	}
	info.AccountID = acct.ID
	info.AccountName = acct.Name
	if fallbackUsed {
		info.Warnings = append(info.Warnings, "通过 /user/memberships 解析账户")
	}

	// 3. Permission probes.
	// Tunnel:Edit via the tunnel list endpoint.
	st, err := c.probe(ctx, "/accounts/"+acct.ID+"/cfd_tunnel?per_page=1", "Tunnel:Edit")
	if err != nil {
		return TokenInfo{}, err
	}
	info.Permissions.TunnelEdit = st

	// Zone:Read via the zones endpoint; also captures the first zone id.
	var zones []Zone
	zerr := c.getJSON(ctx, "/zones?per_page=1", &zones, "Zone:Read")
	switch {
	case zerr == nil && len(zones) > 0:
		info.Permissions.ZoneRead = PermissionOK
		info.FirstZoneID = zones[0].ID
	case zerr == nil:
		// Account has no zones: DNS:Edit cannot be probed yet.
		info.Permissions.ZoneRead = PermissionOK
		info.Permissions.DNSEdit = PermissionUnverified
		info.Warnings = append(info.Warnings, "账户下无 Zone，DNS 权限待绑定域名后验证")
	default:
		if IsAPIError(zerr, KindPermission) {
			info.Permissions.ZoneRead = PermissionMissing
			info.Permissions.DNSEdit = PermissionMissing
		} else {
			return TokenInfo{}, zerr
		}
	}

	// DNS:Edit via the first zone's DNS records endpoint.
	if info.FirstZoneID != "" {
		st, err := c.probe(ctx, "/zones/"+info.FirstZoneID+"/dns_records?per_page=1", "DNS:Edit")
		if err != nil {
			return TokenInfo{}, err
		}
		info.Permissions.DNSEdit = st
	}

	return info, nil
}

// resolveAccount returns the first account the token can see. It tries
// /accounts first and falls back to /user/memberships when /accounts is
// empty or returns 403. When both fail it returns an AuthError.
func (c *client) resolveAccount(ctx context.Context) (Account, bool, error) {
	var accts []Account
	err := c.getJSON(ctx, "/accounts", &accts, "")
	if err == nil {
		if len(accts) > 0 {
			return accts[0], false, nil
		}
		// empty result → fallback
	} else if !IsAPIError(err, KindPermission) {
		return Account{}, false, err
	}

	// Fallback: /user/memberships → result[] is an array of {account:{id,name}}.
	var ms []struct {
		Account Account `json:"account"`
	}
	if err := c.getJSON(ctx, "/user/memberships", &ms, ""); err == nil && len(ms) > 0 {
		return ms[0].Account, true, nil
	}

	return Account{}, false, &APIError{Kind: KindAuth, Message: "无法解析账户，请检查 Token 是否包含 Account 读取权限"}
}

// ListTunnels implements CFClient.ListTunnels. page/perPage default to
// 1/50 when zero or negative.
func (c *client) ListTunnels(ctx context.Context, accountID string, page, perPage int) ([]Tunnel, error) {
	if strings.TrimSpace(accountID) == "" {
		return nil, &APIError{Kind: KindAuth, Message: "账户未解析，请重新认证"}
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	var tunnels []Tunnel
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel?page=%d&per_page=%d", accountID, page, perPage)
	if err := c.getJSON(ctx, path, &tunnels, "Tunnel:Edit"); err != nil {
		return nil, err
	}
	return tunnels, nil
}

// GetTunnelToken implements CFClient.GetTunnelToken.
func (c *client) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(tunnelID) == "" {
		return "", &APIError{Kind: KindAuth, Message: "账户未解析，请重新认证"}
	}
	var tt TunnelToken
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/token", accountID, tunnelID)
	if err := c.getJSON(ctx, path, &tt, "Tunnel:Edit"); err != nil {
		return "", err
	}
	if strings.TrimSpace(tt.Token) == "" {
		return "", &APIError{Kind: KindAPI, Message: "Tunnel 运行 Token 为空"}
	}
	return tt.Token, nil
}
