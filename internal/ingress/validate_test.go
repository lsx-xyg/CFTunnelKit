package ingress

import (
	"testing"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

func rule(hostname, service string) cloudflare.IngressRule {
	return cloudflare.IngressRule{Hostname: hostname, Service: service}
}

func TestValidate_AtLeastOneRule(t *testing.T) {
	errs := Validate(nil, nil)
	if len(errs) != 1 || errs[0].Index != -1 {
		t.Fatalf("errs = %+v, want single whole-list error", errs)
	}
}

func TestValidate_DuplicateHostname(t *testing.T) {
	errs := Validate([]cloudflare.IngressRule{
		rule("nas.example.com", "http://localhost:5000"),
		rule("NAS.EXAMPLE.COM", "http://localhost:5001"),
	}, nil)
	if len(errs) != 1 || errs[0].Field != "hostname" {
		t.Fatalf("errs = %+v, want one hostname error", errs)
	}
}

func TestValidate_WildcardOnlyLeftmost(t *testing.T) {
	ok := Validate([]cloudflare.IngressRule{rule("*.example.com", "http://localhost:8080")}, nil)
	if len(ok) != 0 {
		t.Fatalf("valid wildcard rejected: %+v", ok)
	}
	mid := Validate([]cloudflare.IngressRule{rule("a.*.example.com", "http://localhost:8080")}, nil)
	if len(mid) != 1 || mid[0].Field != "hostname" {
		t.Fatalf("mid-label wildcard not rejected: %+v", mid)
	}
}

func TestValidate_ServiceFormat(t *testing.T) {
	ok := Validate([]cloudflare.IngressRule{rule("nas.example.com", "http_status:404")}, nil)
	if len(ok) != 0 {
		t.Fatalf("http_status:404 rejected: %+v", ok)
	}
	bad := Validate([]cloudflare.IngressRule{rule("nas.example.com", "localhost:5000")}, nil)
	if len(bad) != 1 || bad[0].Field != "service" {
		t.Fatalf("bare localhost:5000 not rejected: %+v", bad)
	}
}

func TestValidate_HostnameRootDomainNotInZones(t *testing.T) {
	zones := []cloudflare.Zone{{ID: "z1", Name: "example.com"}}
	errs := Validate([]cloudflare.IngressRule{rule("nas.other.com", "http://localhost:5000")}, zones)
	if len(errs) != 1 || errs[0].Field != "hostname" || errs[0].Index != 0 {
		t.Fatalf("errs = %+v, want one zone error on index 0", errs)
	}
	// subdomain of an existing zone passes
	if errs := Validate([]cloudflare.IngressRule{rule("a.b.example.com", "http://localhost:5000")}, zones); len(errs) != 0 {
		t.Fatalf("subdomain rejected: %+v", errs)
	}
}

func TestMatchZone_LongestSuffixWins(t *testing.T) {
	zones := []cloudflare.Zone{{ID: "z1", Name: "example.com"}, {ID: "z2", Name: "b.example.com"}}
	if got := MatchZone("a.b.example.com", zones); got != "b.example.com" {
		t.Errorf("MatchZone = %q, want b.example.com", got)
	}
	if got := MatchZone("*.example.com", zones); got != "example.com" {
		t.Errorf("wildcard MatchZone = %q, want example.com", got)
	}
	if got := MatchZone("other.com", zones); got != "" {
		t.Errorf("MatchZone = %q, want empty", got)
	}
}

func TestSameRules_IgnoresCaseAndSpace(t *testing.T) {
	a := []cloudflare.IngressRule{rule("Nas.Example.com", " http://localhost:5000 ")}
	b := []cloudflare.IngressRule{rule("nas.example.com", "http://localhost:5000")}
	if !SameRules(a, b) {
		t.Error("SameRules = false, want true (normalized equal)")
	}
	// order matters
	c := []cloudflare.IngressRule{
		rule("b.example.com", "http://localhost:5001"),
		rule("a.example.com", "http://localhost:5000"),
	}
	if SameRules(b, c) || SameRules(c, b) {
		t.Error("SameRules = true for reordered lists, want false")
	}
}
