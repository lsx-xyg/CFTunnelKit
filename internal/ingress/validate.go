// Package ingress owns the pure validation, zone-matching and
// read-back-comparison functions for ingress rules (issue #5). No HTTP and
// no state: every function is unit-testable in isolation.
package ingress

import (
	"regexp"
	"strings"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

// RuleError is one validation failure bound to a rule index
// (Index -1 means the whole list, e.g. "at least one rule").
type RuleError struct {
	Index int    `json:"index"`
	Field string `json:"field"`
	Msg   string `json:"msg"`
}

// ServiceRe matches the allowed service values:
// http(s)://, tcp://, ssh://, unix://, rdp:// or http_status:<code>.
var ServiceRe = regexp.MustCompile(`^(https?|tcp|ssh|unix|rdp)://|^http_status:\d+$`)

// Validate checks the rules list (issue #5):
//   - at least one non-catch-all rule
//   - hostnames unique (case-insensitive)
//   - wildcard `*` only at the leftmost label (e.g. *.example.com)
//   - hostname root domain belongs to one of zones (longest suffix match)
//   - service matches ServiceRe
//
// An empty zones slice disables the zone check (callers without zone data
// skip it; the frontend greys DNS separately when there are no zones).
func Validate(rules []cloudflare.IngressRule, zones []cloudflare.Zone) []RuleError {
	var errs []RuleError
	if len(rules) == 0 {
		errs = append(errs, RuleError{Index: -1, Field: "rules", Msg: "至少需要一条规则"})
		return errs
	}
	seen := map[string]bool{}
	for i, r := range rules {
		h := strings.TrimSpace(r.Hostname)
		s := strings.TrimSpace(r.Service)

		if h == "" {
			errs = append(errs, RuleError{Index: i, Field: "hostname", Msg: "hostname 不能为空"})
		} else {
			hl := strings.ToLower(h)
			if seen[hl] {
				errs = append(errs, RuleError{Index: i, Field: "hostname", Msg: "hostname 重复：" + h})
			}
			seen[hl] = true

			if strings.Contains(h, "*") && (!strings.HasPrefix(h, "*.") || strings.Count(h, "*") > 1) {
				errs = append(errs, RuleError{Index: i, Field: "hostname", Msg: "通配符 * 仅允许出现在最左侧（如 *.example.com）"})
			}

			if len(zones) > 0 && MatchZone(h, zones) == "" {
				errs = append(errs, RuleError{Index: i, Field: "hostname", Msg: "域名 " + h + " 不在当前账户的 Zone 列表中"})
			}
		}

		if !ServiceRe.MatchString(s) {
			errs = append(errs, RuleError{Index: i, Field: "service", Msg: "service 格式不正确（如 http://localhost:8080 或 http_status:404）"})
		}
	}
	return errs
}

// MatchZone returns the zone whose name is the longest suffix of hostname
// (issue #5/#6). A leading "*." is ignored for matching. Empty when no zone
// matches.
func MatchZone(hostname string, zones []cloudflare.Zone) string {
	h := strings.ToLower(strings.TrimSpace(hostname))
	h = strings.TrimPrefix(h, "*.")
	best := ""
	for _, z := range zones {
		zn := strings.ToLower(strings.TrimSpace(z.Name))
		if h == zn || strings.HasSuffix(h, "."+zn) {
			if len(zn) > len(best) {
				best = zn
			}
		}
	}
	return best
}

// NormalizeRule trims whitespace and lowercases both fields. Used by the
// PUT-then-GET read-back comparison: Cloudflare may normalize formatting,
// so we compare trimmed, lowercased values instead of raw strings.
func NormalizeRule(r cloudflare.IngressRule) cloudflare.IngressRule {
	return cloudflare.IngressRule{
		Hostname: strings.ToLower(strings.TrimSpace(r.Hostname)),
		Service:  strings.ToLower(strings.TrimSpace(r.Service)),
	}
}

// SameRules reports whether two rule lists are equal in content AND order,
// ignoring whitespace and case (issue #5 read-back check after PUT).
func SameRules(a, b []cloudflare.IngressRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if NormalizeRule(a[i]) != NormalizeRule(b[i]) {
			return false
		}
	}
	return true
}
