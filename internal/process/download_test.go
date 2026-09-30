package process

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAssetName(t *testing.T) {
	cases := []struct{ goos, goarch, want string }{
		{"windows", "amd64", "cloudflared-windows-amd64.exe"},
		{"windows", "arm64", ""},
		{"darwin", "amd64", "cloudflared-darwin-amd64.tgz"},
		{"darwin", "arm64", "cloudflared-darwin-arm64.tgz"},
		{"darwin", "386", ""},
		{"linux", "amd64", "cloudflared-linux-amd64"},
		{"linux", "arm64", ""},
		{"freebsd", "amd64", ""},
	}
	for _, c := range cases {
		if got := assetName(c.goos, c.goarch); got != c.want {
			t.Errorf("assetName(%s, %s) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

// fakeBinary returns a body larger than minBinarySize (2MB) so size checks
// pass; tests that need a small body use makeBody(n).
func makeBody(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}

func TestEnsureBinary_DownloadsAndInstalls(t *testing.T) {
	body := makeBody(2 << 20)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Length", itoa(len(body)))
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	var events []map[string]interface{}
	var mu sync.Mutex
	m := NewManager(t.TempDir(), func(event string, payload interface{}) {
		mu.Lock()
		defer mu.Unlock()
		if event == "cloudflared:download" {
			events = append(events, payload.(map[string]interface{}))
		}
	})

	if err := m.EnsureBinary(context.Background()); err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	fi, err := os.Stat(m.BinPath())
	if err != nil {
		t.Fatalf("binary not installed: %v", err)
	}
	if fi.Size() != int64(len(body)) {
		t.Errorf("size = %d, want %d", fi.Size(), len(body))
	}
	if fi.Mode().Perm()&0o111 == 0 {
		t.Error("binary should be executable")
	}
	if hits.Load() != 1 {
		t.Errorf("downloads = %d, want 1", hits.Load())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 {
		t.Fatalf("progress events = %d, want >= 2 (downloading + done)", len(events))
	}
	if events[0]["phase"] != "downloading" {
		t.Errorf("first event phase = %v, want downloading", events[0]["phase"])
	}
	if events[len(events)-1]["phase"] != "done" {
		t.Errorf("last event phase = %v, want done", events[len(events)-1]["phase"])
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestEnsureBinary_AlreadyPresent_NoDownload(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(makeBody(2 << 20))
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	dir := t.TempDir()
	m := NewManager(dir, nil)
	if err := os.WriteFile(m.BinPath(), makeBody(2<<20), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := m.EnsureBinary(context.Background()); err != nil {
		t.Fatalf("EnsureBinary: %v", err)
	}
	if hits.Load() != 0 {
		t.Errorf("downloads = %d, want 0 (already present)", hits.Load())
	}
}

func TestEnsureBinary_RetriesOnceThenFails(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	m := NewManager(t.TempDir(), nil)
	err := m.EnsureBinary(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "下载 cloudflared 失败") {
		t.Errorf("error = %q, want to contain 下载 cloudflared 失败", err.Error())
	}
	if hits.Load() != 2 {
		t.Errorf("downloads = %d, want 2 (retry once)", hits.Load())
	}
	if _, statErr := os.Stat(filepath.Join(m.binDirPath(), "cloudflared.download")); statErr == nil {
		t.Error("temp .download file should be cleaned up")
	}
}

func TestEnsureBinary_SizeTooSmall_Fails(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(makeBody(100)) // below minBinarySize
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	m := NewManager(t.TempDir(), nil)
	err := m.EnsureBinary(context.Background())
	if err == nil {
		t.Fatal("expected error for undersized download")
	}
	if !strings.Contains(err.Error(), "校验失败") {
		t.Errorf("error = %q, want to contain 校验失败", err.Error())
	}
	if hits.Load() != 2 {
		t.Errorf("downloads = %d, want 2 (retry once)", hits.Load())
	}
}

func TestEnsureBinary_Concurrent_OnlyOneDownload(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Length", "2097152")
		_, _ = w.Write(makeBody(2 << 20))
	}))
	defer srv.Close()
	old := releaseBase
	releaseBase = srv.URL
	defer func() { releaseBase = old }()

	m := NewManager(t.TempDir(), nil)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = m.EnsureBinary(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("downloads = %d, want 1 (mutex-guarded)", hits.Load())
	}
}

func TestExtractTgz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.tgz")
	dst := filepath.Join(dir, "cloudflared")

	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	content := makeBody(2 << 20)
	if err := tw.WriteHeader(&tar.Header{Name: "cloudflared", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if err := extractTgz(src, dst); err != nil {
		t.Fatalf("extractTgz: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Error("extracted content mismatch")
	}
	fi, _ := os.Stat(dst)
	if fi.Mode().Perm()&0o111 == 0 {
		t.Error("extracted binary should be executable")
	}
}

// silence unused-import guard for io in case of future refactors
var _ = io.EOF
