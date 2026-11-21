package upgrade

import "context"

// MigrateTRC2 is a maintenance entry point for one known source and target.
// The regular install and update commands continue to require a matching kernel.
func MigrateTRC2(ctx context.Context, assets string) error {
	opts := DefaultOptions()
	opts.Version = "v2.3.1-trc.2"
	return migratePair(ctx, assets, opts, DefaultDependencies(), migrationSourcePair{
		version:   "v2.3.1-trc.1",
		panelSHA:  "3afb4be6361f2ee181738b2040640cefbeb55334c86c996456c5e4445e3a7230",
		kernelSHA: "645d904d672590321e751358eb46e863f18ae64db9490a7f8266932821d25f77",
		paired:    true,
	}, map[string]string{
		BinaryAsset:       "282242f403cb7881f3ef93a3bac7688a2c6e4e6c5c7e4904b6a42f9148ec17f5",
		pairKernelAsset:   "35eb09003de3026294f5f963474163d78600a3f12c546cbeab81dafe16f4baa9",
		pairManifestAsset: "fa27cc173f5608adbf37e27f0103514a9613148635e0e656eef4b1e3cb100498",
		ChecksumAsset:     "1721dc45ca4de8a8fb7b8bf1bc9053f8003f0a7b5ef7eb615d89dc42d4463088",
	})
}
