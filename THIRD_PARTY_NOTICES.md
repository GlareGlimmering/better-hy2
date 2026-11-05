# Copyright and third-party notices

Better HY2 is derived from H UI by jonssonyan and its contributors:
https://github.com/jonssonyan/h-ui

The H UI source and modifications are distributed under GPL-3.0; the original
LICENSE text is retained unmodified. Upstream authorship, source comments and
applicable notices remain in the source. The internal module name remains h-ui.

Hysteria2 (https://github.com/apernet/hysteria) and the independently distributed
custom kernel (https://github.com/GlareGlimmering/hysteria) retain their own source
and license notices. Xray-core (https://github.com/XTLS/Xray-core) is downloaded
separately and retains its source/license notices. Neither proxy kernel is
bundled into the panel binary. No upstream proxy implementation is relicensed
by this panel's branding change.

Frontend dependencies and their transitive packages are pinned by
frontend/pnpm-lock.yaml; Go dependencies are recorded in go.mod/go.sum. Their
individual license and copyright notices continue to apply. Vue, Element Plus,
Vite and other frontend libraries retain the notices emitted in production
bundles. This release does not change or upgrade dependency licenses.
Verbatim notices available in the pinned local dependency trees are collected
in [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt), including build tooling.

The Better HY2 SVG mark in frontend/public/favicon.svg and
frontend/src/assets/logo.svg was created for this project and is licensed under
the same GPL-3.0 terms as the panel. Older upstream images, where retained, remain
subject to their original notices; attribution is not removed by the new mark.
