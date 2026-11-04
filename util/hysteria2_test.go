package util

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"h-ui/model/constant"
)

func TestDownloadHysteria2CustomVersion(t *testing.T) {
	target := filepath.Join(t.TempDir(), "hysteria")
	candidate := []byte("custom-kernel")
	hash := fmt.Sprintf("%x", sha256.Sum256(candidate))
	deps := hysteria2DownloadDependencies{
		assetName: func() (string, error) {
			return "hysteria-linux-amd64-v2.13.0-hui.3", nil
		},
		releaseAssetURL: func(owner, repo, version, fileName string) (string, error) {
			if owner != constant.Hysteria2CustomOwner || repo != constant.Hysteria2CustomRepo || version != constant.Hysteria2CustomVersion {
				t.Fatalf("release = %s/%s %s", owner, repo, version)
			}
			if fileName != "hysteria-linux-amd64-v2.13.0-hui.3" {
				t.Fatalf("asset = %q", fileName)
			}
			return "https://download.example/custom-hysteria", nil
		},
		targetPath:     func() string { return target },
		expectedSHA256: hash,
		downloadAsset: func(url, destination string) error {
			if url != "https://download.example/custom-hysteria" {
				t.Fatalf("download URL = %q", url)
			}
			return os.WriteFile(destination, candidate, 0644)
		},
		versionAt: func(string) (string, error) {
			return constant.Hysteria2CustomVersion, nil
		},
	}
	if err := downloadHysteria2WithDependencies(constant.Hysteria2CustomVersion, deps); err != nil {
		t.Fatalf("downloadHysteria2WithDependencies() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(candidate) {
		t.Fatalf("installed content = %q", content)
	}
}

func TestDownloadHysteria2FailuresPreserveOldBinary(t *testing.T) {
	candidate := []byte("candidate")
	hash := fmt.Sprintf("%x", sha256.Sum256(candidate))
	tests := []struct {
		name         string
		download     func(string, string) error
		expectedHash string
		versionAt    func(string) (string, error)
		wantError    string
	}{
		{
			name: "download failure",
			download: func(string, string) error {
				return errors.New("network unavailable")
			},
			expectedHash: hash,
			versionAt: func(string) (string, error) {
				return constant.Hysteria2CustomVersion, nil
			},
			wantError: "download Hysteria2 candidate: network unavailable",
		},
		{
			name: "SHA-256 mismatch",
			download: func(_ string, destination string) error {
				return os.WriteFile(destination, candidate, 0644)
			},
			expectedHash: "incorrect",
			versionAt: func(string) (string, error) {
				return constant.Hysteria2CustomVersion, nil
			},
			wantError: "SHA-256 mismatch",
		},
		{
			name: "reported version mismatch",
			download: func(_ string, destination string) error {
				return os.WriteFile(destination, candidate, 0644)
			},
			expectedHash: hash,
			versionAt: func(string) (string, error) {
				return "v2.13.0", nil
			},
			wantError: `reports "v2.13.0", expected "v2.13.0-hui.3"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "hysteria")
			if err := os.WriteFile(target, []byte("old-kernel"), 0755); err != nil {
				t.Fatal(err)
			}
			deps := hysteria2DownloadDependencies{
				assetName: func() (string, error) { return "asset", nil },
				releaseAssetURL: func(string, string, string, string) (string, error) {
					return "https://download.example/asset", nil
				},
				targetPath:     func() string { return target },
				expectedSHA256: tt.expectedHash,
				downloadAsset:  tt.download,
				versionAt:      tt.versionAt,
			}
			err := downloadHysteria2WithDependencies(constant.Hysteria2CustomVersion, deps)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(content) != "old-kernel" {
				t.Fatalf("old binary changed to %q", content)
			}
		})
	}
}

func TestInstallHysteria2BinaryReplacesOnlyVerifiedCandidate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hysteria")
	if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	candidate := []byte("custom-kernel")
	hash := fmt.Sprintf("%x", sha256.Sum256(candidate))
	err := installHysteria2Binary(target, hash, func(path string) error {
		return os.WriteFile(path, candidate, 0644)
	}, func(path string) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if string(content) != string(candidate) {
			return errors.New("unexpected candidate")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("installHysteria2Binary() error = %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(candidate) {
		t.Fatalf("installed content = %q", content)
	}
}

func TestInstallHysteria2BinaryPreservesOldOnFailure(t *testing.T) {
	tests := []struct {
		name   string
		hash   string
		verify func(string) error
	}{
		{name: "checksum", hash: "incorrect", verify: func(string) error { return nil }},
		{name: "executable verification", hash: fmt.Sprintf("%x", sha256.Sum256([]byte("candidate"))), verify: func(string) error { return errors.New("bad version") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "hysteria")
			if err := os.WriteFile(target, []byte("old"), 0755); err != nil {
				t.Fatal(err)
			}
			err := installHysteria2Binary(target, tt.hash, func(path string) error {
				return os.WriteFile(path, []byte("candidate"), 0644)
			}, tt.verify)
			if err == nil {
				t.Fatal("invalid candidate was installed")
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(content) != "old" {
				t.Fatalf("old binary changed to %q", content)
			}
		})
	}
}

func TestInstallHysteria2BinaryRestoresOldAfterActivationFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "hysteria")
	if err := os.WriteFile(target, []byte("old-kernel"), 0755); err != nil {
		t.Fatal(err)
	}
	candidate := []byte("candidate")
	hash := fmt.Sprintf("%x", sha256.Sum256(candidate))
	renameCalls := 0
	err := installHysteria2BinaryWithRename(target, hash, func(path string) error {
		return os.WriteFile(path, candidate, 0644)
	}, func(string) error {
		return nil
	}, func(oldPath, newPath string) error {
		renameCalls++
		if renameCalls == 2 {
			return errors.New("activation denied")
		}
		return os.Rename(oldPath, newPath)
	})
	if err == nil || !strings.Contains(err.Error(), "activation denied") {
		t.Fatalf("error = %v", err)
	}
	content, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(content) != "old-kernel" {
		t.Fatalf("old binary changed to %q", content)
	}
}

func TestParseHysteria2VersionPreservesCustomSuffix(t *testing.T) {
	version, err := parseHysteria2Version("Hysteria2 server v2.13.0-hui.3 linux amd64")
	if err != nil {
		t.Fatal(err)
	}
	if version != "v2.13.0-hui.3" {
		t.Fatalf("version = %q", version)
	}
	if _, err := parseHysteria2Version("unknown"); err == nil {
		t.Fatal("unrecognized version output was accepted")
	}
}

func TestDownloadHysteria2RejectsUnpinnedVersion(t *testing.T) {
	err := DownloadHysteria2("v2.13.0")
	if err == nil || !strings.Contains(err.Error(), "unsupported managed Hysteria2 version") {
		t.Fatalf("official version error = %v", err)
	}
}
