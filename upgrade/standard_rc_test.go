package upgrade

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStandardRCOrdering(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0-custom.8-rc.23", "v1.1.0-rc.9", "v1.1.0-rc.10", "v1.1.0"}
	for i, tag := range tags {
		if !releaseTagPattern.MatchString(tag) {
			t.Fatalf("invalid tag %s", tag)
		}
		if i > 0 && compareReleaseTags(tags[i-1], tag) >= 0 {
			t.Fatalf("expected %s before %s", tags[i-1], tag)
		}
	}
	prefix := "HUI_INSTALLER_TEST=1; source " + shellQuote(installerPath(t)) + "; "
	script := prefix + `allowed_version v1.1.0-rc.10 && version_is_older v1.1.0-rc.9 v1.1.0-rc.10 && version_is_older v1.1.0-rc.10 v1.1.0 && select_latest_prerelease '[{"tag_name":"v1.1.0-rc.10"},{"tag_name":"v1.1.0-rc.9"},{"tag_name":"v1.0.0-custom.8-rc.23"}]'`
	output, err := runBash(t, script)
	if err != nil || strings.TrimSpace(output) != "v1.1.0-rc.10" {
		t.Fatalf("installer selection: %v %s", err, output)
	}
	script = prefix + `requested_version=v1.0.0; ensure_no_automatic_downgrade v1.0.0 v1.1.0-rc.1`
	if output, err := runBash(t, script); err == nil || !strings.Contains(output, "matching pre-upgrade snapshot") {
		t.Fatalf("installer accepted incompatible downgrade: %v %s", err, output)
	}
}

func TestStandardRCExplicitStableDowngradeRequiresSnapshot(t *testing.T) {
	f := newUpgradeFixture(t, "")
	f.opts.CurrentVersion = "v1.1.0-rc.1"
	f.opts.Version = stableVersion
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/"+DefaultRepository+"/releases/tags/"+stableVersion {
			http.NotFound(w, r)
			return
		}
		writeRelease(t, w, "", stableVersion, false)
	}))
	defer server.Close()
	f.opts.APIBase = server.URL
	err := Run(context.Background(), f.opts, f.deps)
	if err == nil || !strings.Contains(err.Error(), "matching pre-upgrade snapshot") {
		t.Fatalf("expected snapshot guard, got %v", err)
	}
}
