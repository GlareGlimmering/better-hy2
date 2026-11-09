package service

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/proxy"
)

func TestGeneratedHY2ConfigurationSeparatesTrafficStatsSecret(t *testing.T) {
	setupSubscriptionTestDatabase(t)
	config, err := GetHysteria2Config()
	if err != nil {
		t.Fatal(err)
	}
	data, err := prepareHysteria2ConfigYAML(config)
	if err != nil {
		t.Fatal(err)
	}
	var candidate bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal(data, &candidate); err != nil {
		t.Fatal(err)
	}
	jwt, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.TrafficStats.Secret == nil || *candidate.TrafficStats.Secret == "" || *candidate.TrafficStats.Secret == *jwt.Value {
		t.Fatal("generated configuration lacks an independent Traffic Stats secret")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/online" || r.Header.Get("Authorization") != *candidate.TrafficStats.Secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, rawPort, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseInt(rawPort, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := trafficStatsSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := proxy.NewHysteria2Api(port).Ready(secret); err != nil {
		t.Fatalf("authenticated readiness failed: %v", err)
	}
	if err := proxy.NewHysteria2Api(port).Ready(*jwt.Value); err == nil {
		t.Fatal("readiness accepted the JWT key")
	}
}

func TestRegenerateHY2ConfigurationReplacesLegacySecretAndMode(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX file modes")
	}
	setupSubscriptionTestDatabase(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("", "better-hy2-mode-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	path := constant.Hysteria2ConfigPath
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("legacy fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := setHysteria2ConfigYAML(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("generated mode=%o", info.Mode().Perm())
	}
	config, err := GetHysteria2Config()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := trafficStatsSecret()
	if err != nil || config.TrafficStats.Secret == nil || *config.TrafficStats.Secret != secret {
		t.Fatal("regeneration failed to persist the generated Traffic Stats secret")
	}
	t.Run("low privilege process", func(t *testing.T) {
		if runtime.GOOS != "linux" || os.Geteuid() != 0 {
			t.Skip("switching to the nobody test user requires local root")
		}
		runuser, err := exec.LookPath("runuser")
		if err != nil {
			t.Skip("runuser unavailable")
		}
		control := filepath.Join(dir, "public-fixture")
		if err := os.WriteFile(control, []byte("readable"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := exec.Command(runuser, "-u", "nobody", "--", "test", "-r", control).Run(); err != nil {
			t.Fatalf("low-privilege control file was not readable: %v", err)
		}
		err = exec.Command(runuser, "-u", "nobody", "--", "test", "-r", filepath.Join(dir, path)).Run()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("low-privilege process did not report access denied: %v", err)
		}
	})
}

func TestHY2RollbackReplacesLegacyPublicConfigWithPrivateFile(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("POSIX file modes")
	}
	path := filepath.Join(t.TempDir(), "hysteria2.yaml")
	if err := os.WriteFile(path, []byte("previous config"), 0644); err != nil {
		t.Fatal(err)
	}
	ops := hysteria2ConfigApplyOps{path: path, writeFile: atomicWriteFile, writeDB: func([]byte) error { return nil }}
	if err := rollbackHysteria2Config(nil, []byte("previous config"), true, false, ops); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("rollback restored unsafe permissions %o", info.Mode().Perm())
	}
}
