package upgrade

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	pairManifestAsset = "HY2-PAIR.json"
	pairKernelAsset   = "hysteria-linux-amd64"
)

var (
	standardRCTag     = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[0-9]+$`)
	pairCommit        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	pairChecksum      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	pairKernelVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+-hui\.[0-9]+([.-][a-z0-9]+)*$`)
)

type pairArtifact struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	SHA256  string `json:"sha256"`
}

type releasePair struct {
	Panel              pairArtifact `json:"panel"`
	Kernel             pairArtifact `json:"kernel"`
	OS                 string       `json:"os"`
	Arch               string       `json:"arch"`
	AccountConsistency int          `json:"accountConsistency"`
	AccountBandwidth   int          `json:"accountBandwidth"`
}

func (p releasePair) validate(tag string) error {
	if p.Panel.Version != tag || !pairKernelVersion.MatchString(p.Kernel.Version) ||
		!pairCommit.MatchString(p.Panel.Commit) || !pairCommit.MatchString(p.Kernel.Commit) ||
		!pairChecksum.MatchString(p.Panel.SHA256) || !pairChecksum.MatchString(p.Kernel.SHA256) ||
		p.OS != "linux" || p.Arch != "amd64" || p.AccountConsistency != 2 || p.AccountBandwidth != 1 {
		return errors.New("invalid or incompatible paired release manifest")
	}
	return nil
}

func preflightReleasePair(ctx context.Context, release releaseInfo, panel, sums, dir string, deps Dependencies) error {
	assets := map[string]string{}
	for _, asset := range release.Assets {
		if asset.Name != pairManifestAsset && asset.Name != pairKernelAsset {
			continue
		}
		if assets[asset.Name] != "" {
			return errors.New("duplicate paired release asset")
		}
		assets[asset.Name] = asset.URL
	}
	if assets[pairManifestAsset] == "" || assets[pairKernelAsset] == "" {
		return errors.New("release lacks verified paired kernel assets")
	}
	manifest := filepath.Join(dir, pairManifestAsset)
	if err := downloadFile(ctx, assets[pairManifestAsset], manifest, deps); err != nil {
		return fmt.Errorf("download paired release manifest: %w", err)
	}
	if err := verifyPairChecksum(manifest, sums); err != nil {
		return err
	}
	file, err := os.Open(manifest)
	if err != nil {
		return err
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() > 4096 {
		return errors.New("invalid paired release manifest size")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var pair releasePair
	if err := decoder.Decode(&pair); err != nil {
		return errors.New("invalid paired release manifest")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid trailing content in paired release manifest")
	}
	if err := pair.validate(release.TagName); err != nil {
		return err
	}
	for name, digest := range map[string]string{BinaryAsset: pair.Panel.SHA256, pairKernelAsset: pair.Kernel.SHA256} {
		expected, err := expectedChecksum(sums, name)
		if err != nil || expected != digest {
			return fmt.Errorf("manifest and SHA256SUMS disagree for %s", name)
		}
	}
	if err := verifyPairExecutable(panel); err != nil {
		return fmt.Errorf("panel candidate: %w", err)
	}
	if err := verifyPanelBuildCommit(panel, pair.Panel.Commit); err != nil {
		return err
	}
	kernel := filepath.Join(dir, pairKernelAsset)
	if err := downloadFile(ctx, assets[pairKernelAsset], kernel, deps); err != nil {
		return fmt.Errorf("download paired kernel: %w", err)
	}
	if err := verifyPairChecksum(kernel, sums); err != nil {
		return err
	}
	if err := verifyPairExecutable(kernel); err != nil {
		return fmt.Errorf("kernel candidate: %w", err)
	}
	if err := os.Chmod(kernel, 0700); err != nil {
		return err
	}
	output, err := deps.Command(kernel, "version", "--json")
	if err != nil {
		return errors.New("kernel candidate did not provide build and capability information")
	}
	var info struct {
		Version            string `json:"version"`
		Commit             string `json:"commit"`
		OS                 string `json:"os"`
		Arch               string `json:"arch"`
		AccountConsistency int    `json:"accountConsistency"`
		AccountBandwidth   int    `json:"accountBandwidth"`
	}
	if json.Unmarshal(output, &info) != nil || info.Version != pair.Kernel.Version || info.Commit != pair.Kernel.Commit ||
		info.OS != pair.OS || info.Arch != pair.Arch || info.AccountConsistency != pair.AccountConsistency ||
		info.AccountBandwidth != pair.AccountBandwidth {
		return errors.New("kernel build or capabilities do not match release manifest")
	}
	return nil
}

func verifyPairChecksum(path, sums string) error {
	name := filepath.Base(path)
	want, err := expectedChecksum(sums, name)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if !strings.EqualFold(fmt.Sprintf("%x", h.Sum(nil)), want) {
		return fmt.Errorf("SHA256 mismatch for %s", name)
	}
	return nil
}

func verifyPairExecutable(path string) error {
	f, err := elf.Open(path)
	if err != nil {
		return errors.New("not an ELF executable")
	}
	defer f.Close()
	if f.Machine != elf.EM_X86_64 || f.Class != elf.ELFCLASS64 ||
		(f.Type != elf.ET_EXEC && f.Type != elf.ET_DYN) {
		return errors.New("expected Linux AMD64 executable")
	}
	return nil
}

func verifyPanelBuildCommit(path, expected string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return errors.New("panel build information unavailable")
	}
	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision != expected || modified != "false" {
		return errors.New("panel build commit does not match manifest or source was modified")
	}
	return nil
}
