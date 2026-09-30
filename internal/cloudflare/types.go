package cloudflare

import "time"

// PermissionStatus is the result of a permission probe.
type PermissionStatus string

const (
	// PermissionOK means the probe endpoint returned 200.
	PermissionOK PermissionStatus = "ok"
	// PermissionMissing means the probe endpoint returned 403.
	PermissionMissing PermissionStatus = "missing"
	// PermissionUnverified means the permission could not be probed
	// (e.g. no Zone exists on the account to probe DNS:Edit against).
	PermissionUnverified PermissionStatus = "unverified"
)

// Permissions carries the three permission probe results required by the spec.
type Permissions struct {
	TunnelEdit PermissionStatus `json:"tunnel_edit"`
	ZoneRead   PermissionStatus `json:"zone_read"`
	DNSEdit    PermissionStatus `json:"dns_edit"`
}

// TokenInfo is the result of VerifyToken: token identity, resolved default
// account, permission probe results and non-fatal warnings.
type TokenInfo struct {
	TokenID     string      `json:"token_id"`
	AccountID   string      `json:"account_id"`
	AccountName string      `json:"account_name"`
	Permissions Permissions `json:"permissions"`
	FirstZoneID string      `json:"first_zone_id"`
	Warnings    []string    `json:"warnings"`
}

// Tunnel is the minimal tunnel record needed by the list view.
type Tunnel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Account is a Cloudflare account as returned by /accounts or
// /user/memberships (result[].account).
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Zone is a Cloudflare zone as returned by /zones.
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
