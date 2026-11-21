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

// Major version 2 uses the account consistency and bandwidth kernel pair.
func usesPairedStableLine(tag string) bool {
	return strings.HasPrefix(tag, "v2.")
}

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

func preflightReleasePair(ctx context.Context, release releaseInfo, panel, sums, dir string, deps Dependencies) (string, error) {
	assets := map[string]string{}
	for _, asset := range release.Assets {
		if asset.Name != pairManifestAsset && asset.Name != pairKernelAsset {
			continue
		}
		if assets[asset.Name] != "" {
			return "", errors.New("duplicate paired release asset")
		}
		assets[asset.Name] = asset.URL
	}
	if assets[pairManifestAsset] == "" || assets[pairKernelAsset] == "" {
		return "", errors.New("release lacks verified paired kernel assets")
	}
	manifest := filepath.Join(dir, pairManifestAsset)
	if err := downloadFile(ctx, assets[pairManifestAsset], manifest, deps); err != nil {
		return "", fmt.Errorf("download paired release manifest: %w", err)
	}
	kernel := filepath.Join(dir, pairKernelAsset)
	if err := downloadFile(ctx, assets[pairKernelAsset], kernel, deps); err != nil {
		return "", fmt.Errorf("download paired kernel: %w", err)
	}
	return verifyReleasePairFiles(release.TagName, panel, sums, manifest, kernel, deps)
}

func verifyReleasePairFiles(tag, panel, sums, manifest, kernel string, deps Dependencies) (string, error) {
	pair, err := readReleasePair(tag, sums, manifest)
	if err != nil {
		return "", err
	}
	if err := verifyPairExecutable(panel); err != nil {
		return "", fmt.Errorf("panel candidate: %w", err)
	}
	if err := verifyPanelBuildCommit(panel, pair.Panel.Commit); err != nil {
		return "", err
	}
	if err := verifyPairChecksum(kernel, sums); err != nil {
		return "", err
	}
	if err := verifyPairExecutable(kernel); err != nil {
		return "", fmt.Errorf("kernel candidate: %w", err)
	}
	if err := os.Chmod(kernel, 0700); err != nil {
		return "", err
	}
	output, err := deps.Command(kernel, "version", "--json")
	if err != nil {
		return "", errors.New("kernel candidate did not provide build and capability information")
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
		return "", errors.New("kernel build or capabilities do not match release manifest")
	}
	return pair.Kernel.SHA256, nil
}

// readReleasePair verifies the manifest and its binary digests against SHA256SUMS.
func readReleasePair(tag, sums, manifest string) (releasePair, error) {
	if err := verifyPairChecksum(manifest, sums); err != nil {
		return releasePair{}, err
	}
	file, err := os.Open(manifest)
	if err != nil {
		return releasePair{}, err
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() > 4096 {
		return releasePair{}, errors.New("invalid paired release manifest size")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var pair releasePair
	if err := decoder.Decode(&pair); err != nil {
		return releasePair{}, errors.New("invalid paired release manifest")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return releasePair{}, errors.New("invalid trailing content in paired release manifest")
	}
	if err := pair.validate(tag); err != nil {
		return releasePair{}, err
	}
	for name, digest := range map[string]string{BinaryAsset: pair.Panel.SHA256, pairKernelAsset: pair.Kernel.SHA256} {
		expected, err := expectedChecksum(sums, name)
		if err != nil || expected != digest {
			return releasePair{}, fmt.Errorf("manifest and SHA256SUMS disagree for %s", name)
		}
	}
	return pair, nil
}

func readInstalledPair(ctx context.Context, opts Options, dir string, deps Dependencies) (migrationSourcePair, error) {
	source := opts
	source.Version = opts.CurrentVersion
	release, err := resolveRelease(ctx, source, deps)
	if err != nil {
		return migrationSourcePair{}, err
	}
	if release.TagName != source.Version || release.Draft || release.Prerelease != isPrereleaseTag(source.Version) {
		return migrationSourcePair{}, errors.New("installed release identity does not match its published channel")
	}
	sourceDir := filepath.Join(dir, "source")
	if err = os.Mkdir(sourceDir, 0700); err != nil {
		return migrationSourcePair{}, err
	}
	for _, name := range []string{ChecksumAsset, pairManifestAsset} {
		var locator string
		for _, asset := range release.Assets {
			if asset.Name == name {
				if locator != "" {
					return migrationSourcePair{}, fmt.Errorf("duplicate source asset %s", name)
				}
				locator = asset.URL
			}
		}
		if locator == "" {
			return migrationSourcePair{}, fmt.Errorf("installed release has no %s", name)
		}
		if err := downloadFile(ctx, locator, filepath.Join(sourceDir, name), deps); err != nil {
			return migrationSourcePair{}, err
		}
	}
	pair, err := readReleasePair(source.Version, filepath.Join(sourceDir, ChecksumAsset), filepath.Join(sourceDir, pairManifestAsset))
	if err != nil {
		return migrationSourcePair{}, err
	}
	return migrationSourcePair{
		version: source.Version, panelSHA: pair.Panel.SHA256, kernelSHA: pair.Kernel.SHA256, paired: true,
	}, nil
}

func verifyPairChecksum(path, sums string) error {
	want, err := expectedChecksum(sums, filepath.Base(path))
	if err != nil {
		return err
	}
	return verifyFileDigest(path, want)
}

func verifyFileDigest(path, want string) error {
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
		return fmt.Errorf("SHA256 mismatch for %s", filepath.Base(path))
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
