package dns

import (
	"context"
	"errors"
	"testing"

	"github.com/lsx-xyg/CFTunnelKit/internal/cloudflare"
)

type stubClient struct {
	records []cloudflare.DNSRecord
	created []cloudflare.DNSRecord
	deleted []string
	createErr error
	listErr   error
	deleteErr error
}

func (s *stubClient) ListDNSRecords(ctx context.Context, zoneID string) ([]cloudflare.DNSRecord, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.records, nil
}

func (s *stubClient) CreateCNAMERecord(ctx context.Context, zoneID, name, target string) (cloudflare.DNSRecord, error) {
	if s.createErr != nil {
		return cloudflare.DNSRecord{}, s.createErr
	}
	rec := cloudflare.DNSRecord{ID: "new-1", Type: "CNAME", Name: name, Content: target, Proxied: true}
	s.created = append(s.created, rec)
	return rec, nil
}

func (s *stubClient) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleted = append(s.deleted, recordID)
	return nil
}

func TestEnsureCNAME_CreatesWhenAbsent(t *testing.T) {
	cl := &stubClient{}
	res, err := EnsureCNAME(context.Background(), cl, "z1", "nas", "tun1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME: %v", err)
	}
	if !res.Created || res.RecordID != "new-1" {
		t.Errorf("res = %+v, want created new-1", res)
	}
	if len(cl.created) != 1 {
		t.Errorf("created = %d, want 1", len(cl.created))
	}
}

func TestEnsureCNAME_IdempotentWhenAlreadyPointingAtTunnel(t *testing.T) {
	cl := &stubClient{records: []cloudflare.DNSRecord{
		{ID: "r1", Type: "CNAME", Name: "Nas", Content: "TUN1.cfargotunnel.com."},
	}}
	res, err := EnsureCNAME(context.Background(), cl, "z1", "nas", "tun1.cfargotunnel.com")
	if err != nil {
		t.Fatalf("EnsureCNAME: %v", err)
	}
	if res.Created || res.RecordID != "r1" {
		t.Errorf("res = %+v, want idempotent r1", res)
	}
	if len(cl.created) != 0 {
		t.Errorf("created = %d, want 0 (no duplicate)", len(cl.created))
	}
}

func TestEnsureCNAME_Taken_WhenPointsElsewhere(t *testing.T) {
	cl := &stubClient{records: []cloudflare.DNSRecord{
		{ID: "r1", Type: "CNAME", Name: "nas", Content: "other.cfargotunnel.com"},
	}}
	_, err := EnsureCNAME(context.Background(), cl, "z1", "nas", "tun1.cfargotunnel.com")
	if !errors.Is(err, ErrTaken) {
		t.Fatalf("error = %v, want ErrTaken", err)
	}
	if !errors.Is(err, ErrTaken) || len(cl.created) != 0 {
		t.Errorf("must not overwrite: created = %d", len(cl.created))
	}
}

func TestEnsureCNAME_Taken_WhenTypeIsNotCNAME(t *testing.T) {
	cl := &stubClient{records: []cloudflare.DNSRecord{
		{ID: "r1", Type: "A", Name: "nas", Content: "1.2.3.4"},
	}}
	_, err := EnsureCNAME(context.Background(), cl, "z1", "nas", "tun1.cfargotunnel.com")
	if !errors.Is(err, ErrTaken) {
		t.Fatalf("error = %v, want ErrTaken", err)
	}
}

func TestDeleteByName_DeletesMatching(t *testing.T) {
	cl := &stubClient{records: []cloudflare.DNSRecord{
		{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"},
		{ID: "r2", Type: "CNAME", Name: "other", Content: "x"},
	}}
	deleted, err := DeleteByName(context.Background(), cl, "z1", "nas")
	if err != nil {
		t.Fatalf("DeleteByName: %v", err)
	}
	if !deleted || len(cl.deleted) != 1 || cl.deleted[0] != "r1" {
		t.Errorf("deleted=%v ids=%v, want true [r1]", deleted, cl.deleted)
	}
}

func TestDeleteByName_Missing_Idempotent(t *testing.T) {
	cl := &stubClient{}
	deleted, err := DeleteByName(context.Background(), cl, "z1", "nope")
	if err != nil {
		t.Fatalf("DeleteByName: %v", err)
	}
	if deleted {
		t.Error("deleted = true, want false for missing record")
	}
}

func TestDeleteByName_ErrorPassthrough(t *testing.T) {
	cl := &stubClient{records: []cloudflare.DNSRecord{
		{ID: "r1", Type: "CNAME", Name: "nas", Content: "tun1.cfargotunnel.com"},
	}, deleteErr: errors.New("403")}
	_, err := DeleteByName(context.Background(), cl, "z1", "nas")
	if err == nil {
		t.Fatal("error = nil, want delete error passthrough")
	}
}
