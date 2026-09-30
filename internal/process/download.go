package process

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// releaseBase is the GitHub Releases "latest" download base for
// cloudflared. Tests replace it with an httptest server URL.
var releaseBase = "https://github.com/cloudflare/cloudflared/releases/latest/download"

// assetName maps GOOS/GOARCH to the cloudflared release asset name
// (issue #5). Unsupported combos return "".
func assetName(goos, goarch string) string {
	switch goos {
	case "windows":
		if goarch != "amd64" {
			return ""
		}
		return "cloudflared-windows-amd64.exe"
	case "darwin":
		switch goarch {
		case "amd64":
			return "cloudflared-darwin-amd64.tgz"
		case "arm64":
			return "cloudflared-darwin-arm64.tgz"
		}
		return ""
	case "linux":
		if goarch != "amd64" {
			return ""
		}
		return "cloudflared-linux-amd64"
	default:
		return ""
	}
}

// EnsureBinary makes sure a valid cloudflared binary exists at BinPath.
// It is idempotent and mutex-guarded: concurrent callers never trigger
// two downloads. On failure it retries once and returns a
// DownloadError-style message for the frontend.
func (m *Manager) EnsureBinary(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if statSize(m.BinPath()) >= minBinarySize {
		return nil // already present and plausibly valid
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := m.downloadOnce(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("下载 cloudflared 失败，请检查网络: %w", lastErr)
}

// downloadOnce downloads (and extracts, for .tgz) the binary into place.
func (m *Manager) downloadOnce(ctx context.Context) error {
	name := assetName(runtime.GOOS, runtime.GOARCH)
	if name == "" {
		return fmt.Errorf("当前平台 %s/%s 暂不支持自动下载", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(m.binDirPath(), 0o755); err != nil {
		return err
	}

	tmp := filepath.Join(m.binDirPath(), "cloudflared.download")
	_ = os.Remove(tmp) // clear a leftover from a previous failed attempt

	m.emit("cloudflared:download", map[string]interface{}{
		"phase": "downloading", "downloaded": int64(0), "total": int64(0),
	})
	if err := m.fetch(ctx, releaseBase+"/"+name, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if size := statSize(tmp); size < minBinarySize {
		_ = os.Remove(tmp)
		return fmt.Errorf("下载文件校验失败（大小 %d 字节，预期 ≥ %d）", size, minBinarySize)
	}

	dest := m.BinPath()
	if strings.HasSuffix(name, ".tgz") {
		m.emit("cloudflared:download", map[string]interface{}{
			"phase": "extracting", "downloaded": int64(0), "total": int64(0),
		})
		if err := extractTgz(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		_ = os.Remove(tmp)
		if statSize(dest) < minBinarySize {
			return fmt.Errorf("解压产物校验失败")
		}
	} else {
		if err := os.Rename(tmp, dest); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		_ = os.Chmod(dest, 0o755)
	}
	m.emit("cloudflared:download", map[string]interface{}{
		"phase": "done", "downloaded": int64(0), "total": int64(0),
	})
	return nil
}

// fetch downloads url to path with progress events and a generous timeout.
func (m *Manager) fetch(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CFTunnelKit/0.1")

	// GitHub /releases/latest/download/ 302-redirects to
	// objects.githubusercontent.com; the default client follows it.
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回 HTTP %d", resp.StatusCode)
	}

	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	pr := &progressReader{
		r:     resp.Body,
		total: resp.ContentLength,
		emit:  m.emitter,
	}
	_, err = io.Copy(out, pr)
	return err
}

// progressReader emits download progress events while copying, throttled
// to one event per 256KB so a 30-50MB download doesn't flood the UI.
type progressReader struct {
	r     io.Reader
	total int64
	done  int64
	last  int64
	emit  Emitter
}

const progressThrottle = 256 * 1024

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.emit != nil && (p.done-p.last >= progressThrottle || err == io.EOF) {
		p.last = p.done
		p.emit("cloudflared:download", map[string]interface{}{
			"phase": "downloading", "downloaded": p.done, "total": p.total,
		})
	}
	return n, err
}

func (m *Manager) emit(event string, payload interface{}) {
	if m.emitter != nil {
		m.emitter(event, payload)
	}
}

// extractTgz extracts the first regular file from a cloudflared .tgz
// release into dst with executable permissions.
func extractTgz(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	}
	return fmt.Errorf("tgz 中未找到可执行文件")
}
