package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/lsx-xyg/CFTunnelKit/internal/version"
)

// UpdateInfo is the result of checking GitHub for a newer release.
type UpdateInfo struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	HasUpdate  bool   `json:"hasUpdate"`
	ReleaseURL string `json:"releaseUrl"`
}

const githubAPI = "https://api.github.com/repos/lsx-xyg/CFTunnelKit/releases/latest"

// CheckLatestRelease fetches the latest GitHub release and compares it
// with the running version.
func (h *SystemHandler) CheckLatestRelease() (*UpdateInfo, error) {
	current := version.Version
	// Local dev build: skip comparison.
	if current == "" || current == "dev" {
		return &UpdateInfo{Current: current, Latest: current, HasUpdate: false, ReleaseURL: ""}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", githubAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CFTunnelKit")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	latest := strings.TrimSpace(release.TagName)
	hasUpdate := compareVersions(current, latest) < 0

	return &UpdateInfo{
		Current:    current,
		Latest:     latest,
		HasUpdate:  hasUpdate,
		ReleaseURL: release.HTMLURL,
	}, nil
}

// OpenReleasePage opens the release URL in the default browser.
func (h *SystemHandler) OpenReleasePage(url string) {
	if h.ctx == nil {
		return
	}
	wailsruntime.BrowserOpenURL(h.ctx, url)
}

// compareVersions returns -1 if a < b, 0 if equal, 1 if a > b.
// Handles optional leading "v". Non-numeric parts are treated as 0.
func compareVersions(a, b string) int {
	pa := parseVer(a)
	pb := parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseVer(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		var n int
		fmt.Sscanf(parts[i], "%d", &n)
		out[i] = n
	}
	return out
}
