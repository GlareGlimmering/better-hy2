package util

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"h-ui/model/constant"
)

func GetHysteria2BinPath() string {
	return constant.BinDir + GetHysteria2BinName()
}

func GetHysteria2BinName() string {
	hysteria2FileName := fmt.Sprintf("hysteria-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		hysteria2FileName += ".exe"
	}
	return hysteria2FileName
}

func GetCustomHysteria2AssetName() (string, error) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return "", errors.New("the managed custom Hysteria2 kernel is available only for linux/amd64")
	}
	return fmt.Sprintf("hysteria-linux-amd64-%s", constant.Hysteria2CustomVersion), nil
}

// DownloadHysteria2 installs only the pinned H-UI kernel. Existing official
// binaries remain usable for legacy mode, but automatic downloads can never
// silently replace the custom kernel with an upstream latest release.
func DownloadHysteria2(version string) error {
	return downloadHysteria2WithDependencies(version, hysteria2DownloadDependencies{
		assetName:       GetCustomHysteria2AssetName,
		releaseAssetURL: GetReleaseAssetURL,
		targetPath:      GetHysteria2BinPath,
		expectedSHA256:  constant.Hysteria2CustomSHA256,
		downloadAsset:   downloadHysteria2Asset,
		versionAt:       Hysteria2VersionAt,
	})
}

type hysteria2DownloadDependencies struct {
	assetName       func() (string, error)
	releaseAssetURL func(owner, repo, version, fileName string) (string, error)
	targetPath      func() string
	expectedSHA256  string
	downloadAsset   func(url, destination string) error
	versionAt       func(path string) (string, error)
}

func downloadHysteria2WithDependencies(version string, deps hysteria2DownloadDependencies) error {
	if version == "" {
		version = constant.Hysteria2CustomVersion
	}
	if version != constant.Hysteria2CustomVersion {
		return fmt.Errorf("unsupported managed Hysteria2 version %q", version)
	}
	assetName, err := deps.assetName()
	if err != nil {
		return fmt.Errorf("select custom Hysteria2 asset: %w", err)
	}
	url, err := deps.releaseAssetURL(constant.Hysteria2CustomOwner, constant.Hysteria2CustomRepo, version, assetName)
	if err != nil {
		return fmt.Errorf("resolve custom Hysteria2 release %s: %w", version, err)
	}
	target := deps.targetPath()
	if err := installHysteria2Binary(target, deps.expectedSHA256, func(path string) error {
		return deps.downloadAsset(url, path)
	}, func(path string) error {
		installed, err := deps.versionAt(path)
		if err != nil {
			return err
		}
		if installed != constant.Hysteria2CustomVersion {
			return fmt.Errorf("downloaded Hysteria2 reports %q, expected %q", installed, constant.Hysteria2CustomVersion)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("install custom Hysteria2 %s: %w", version, err)
	}
	return nil
}

func downloadHysteria2Asset(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download Hysteria2: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download Hysteria2: HTTP %d", resp.StatusCode)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, resp.Body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func installHysteria2Binary(target, expectedHash string, download func(string) error, verify func(string) error) error {
	return installHysteria2BinaryWithRename(target, expectedHash, download, verify, os.Rename)
}

func installHysteria2BinaryWithRename(target, expectedHash string, download func(string) error, verify func(string) error, rename func(string, string) error) error {
	if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
		return fmt.Errorf("create Hysteria2 binary directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".hysteria-install-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		return err
	}
	defer os.Remove(tempPath)
	if err := download(tempPath); err != nil {
		return fmt.Errorf("download Hysteria2 candidate: %w", err)
	}
	content, err := os.ReadFile(tempPath)
	if err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(content))
	if !strings.EqualFold(digest, expectedHash) {
		return fmt.Errorf("Hysteria2 SHA-256 mismatch: got %s, expected %s", digest, expectedHash)
	}
	if err := os.Chmod(tempPath, 0755); err != nil {
		return err
	}
	if err := verify(tempPath); err != nil {
		return fmt.Errorf("verify downloaded Hysteria2: %w", err)
	}

	backup := target + ".previous"
	_ = os.Remove(backup)
	hadOld := Exists(target)
	if hadOld {
		if err := rename(target, backup); err != nil {
			return fmt.Errorf("backup existing Hysteria2: %w", err)
		}
	}
	if err := rename(tempPath, target); err != nil {
		if hadOld {
			if restoreErr := rename(backup, target); restoreErr != nil {
				return fmt.Errorf("activate Hysteria2: %v; restore previous Hysteria2: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("activate Hysteria2: %w", err)
	}
	if hadOld {
		_ = os.Remove(backup)
	}
	return nil
}

func Hysteria2VersionAt(path string) (string, error) {
	output, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("execute Hysteria2 version: %w", err)
	}
	return parseHysteria2Version(string(output))
}

func parseHysteria2Version(output string) (string, error) {
	pattern := regexp.MustCompile(`v\d+\.\d+\.\d+(?:-hui\.\d+(?:-dev)?)?`)
	version := pattern.FindString(output)
	if version == "" {
		return "", errors.New("Hysteria2 version output is not recognized")
	}
	return version, nil
}

func InstalledHysteria2Version() (string, error) {
	return Hysteria2VersionAt(GetHysteria2BinPath())
}
