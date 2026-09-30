// Package dns owns the idempotent CNAME ensure / delete logic behind the
// DNS-link features (issue #6). The Cloudflare HTTP calls are behind a small
// Client interface so every branch (idempotent, conflict, create, delete)
// is unit-testable with a stub.
package dns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// Client is the DNS subset of CFClient the ensure/delete logic needs.
type Client interface {
	ListDNSRecords(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error)
	CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (cloudflare.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error
}

// ErrTaken is returned when a same-name record already exists that does not
// point at the requested target (or is not a CNAME at all): never overwrite,
// only tell the user (issue #6 conflict rule).
var ErrTaken = errors.New("taken")

// normalizeRecordKey trims whitespace, lowercases and drops the trailing
// dot for record name / content comparisons.
func normalizeRecordKey(s string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "."))
}

// EnsureCNAME makes sure a CNAME record name → target exists in the zone
// (issue #6):
//   - same-name CNAME already pointing at target → idempotent success,
//     Created=false
//   - same-name record with a different target, or not a CNAME → ErrTaken
//   - no same-name record → created, Created=true
func EnsureCNAME(ctx context.Context, cl Client, zoneID, name, target string) (cloudflare.DNSEnsureResult, error) {
	records, err := cl.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return cloudflare.DNSEnsureResult{}, err
	}
	wantName := normalizeRecordKey(name)
	wantTarget := normalizeRecordKey(target)
	for _, r := range records {
		if normalizeRecordKey(r.Name) != wantName {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(r.Type), "CNAME") && normalizeRecordKey(r.Content) == wantTarget {
			return cloudflare.DNSEnsureResult{RecordID: r.ID, Created: false}, nil
		}
		return cloudflare.DNSEnsureResult{}, fmt.Errorf("%w: 域名 %s 已被占用，请手动处理", ErrTaken, strings.TrimSpace(name))
	}
	rec, err := cl.CreateCNAMERecord(ctx, zoneID, name, target)
	if err != nil {
		return cloudflare.DNSEnsureResult{}, err
	}
	return cloudflare.DNSEnsureResult{RecordID: rec.ID, Created: true}, nil
}

// DeleteByName removes the record whose name matches in the zone (issue #6
// delete link). A missing record is an idempotent success (deleted=false).
func DeleteByName(ctx context.Context, cl Client, zoneID, name string) (bool, error) {
	records, err := cl.ListDNSRecords(ctx, zoneID)
	if err != nil {
		return false, err
	}
	wantName := normalizeRecordKey(name)
	for _, r := range records {
		if normalizeRecordKey(r.Name) == wantName {
			if err := cl.DeleteDNSRecord(ctx, zoneID, r.ID); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}
