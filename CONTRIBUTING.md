# Contributing

Thanks for helping. Bug reports, docs fixes and features are all welcome.

## Before you start

For anything bigger than a small fix, please open an
[issue](https://github.com/al-maisan/gmt-mail/issues) first so we can agree on
the approach.

## Development

Requires [Go](https://go.dev/) 1.26 or later.

```sh
make build         # ./gmt-mail with version, commit and build date
make test          # unit tests
make lint          # go vet + golangci-lint
make fmt           # gofmt
make test-pkg      # tests of the packaging and release scripts (needs dpkg-deb)
make test-image    # builds and runs a test container image; skipped without podman or docker
make man           # renders the man page for viewing
```

CI runs `make vet`, golangci-lint, `make test`, `make test-pkg` and
`make build` on every pull request.

All targets: `all`, `build`, `test`, `vet`, `lint`, `fmt`, `install`, `vendor`, `clean`, `srpm`, `copr`, `ppa`, `test-ppa`, `test-copr`, `man`, `test-man`, `tag`, `dist`, `test-dist`, `release`, `brew`, `test-brew`, `doc-version`, `test-release`, `test-pkg`, `image`, `push-image`, `test-image`, `publish`.

A new flag also goes into the man page, `gmt-mail.1`, and the CLI reference
in `docs/usage.md`. New behaviour comes with tests, and docs change in the same
pull request.

## Releasing

The version is derived from the latest git tag — there is no `VERSION` to edit.
Cut a release by setting the version of the download example in `docs/install.md` (`make doc-version V=0.7.0`, then merge that change), creating and pushing a signed tag, then publishing everywhere:

    $ git tag -s v0.7.0 -m "v0.7.0"
    $ git push origin v0.7.0
    $ make publish

`make publish` creates the GitHub release with prebuilt archives, submits to Fedora COPR, uploads to Ubuntu PPA and bumps the Homebrew formula, all using the tagged version. To cut it in one shot without tagging first, override: `make publish VERSION=0.7.0` creates and pushes the tag for you.

> **Note:** both package changelogs (the RPM `%changelog` and the Debian `debian/changelog`) are generated from `git log` between **GitHub releases**, so a version appears in them only once a GitHub *release* exists for its tag. `make publish` (and `make release`) create the release *before* building packages, so a full `make publish` yields complete changelogs — but building packages standalone (`make srpm`/`make copr`/`make ppa`) before the release exists will omit that version.

Before it builds or uploads anything, `make release` runs `scripts/release-check.sh`: it stops unless the tree is clean, HEAD is the tag, the tag is `vMAJOR.MINOR.PATCH[-PRE]`, signed by the release key (`RELEASE_KEY`, default `753B6ECF2B458FF3D19D568C1E0A288397AE739E`) and the same tag object on GitHub, and HEAD is on `master` of GitHub. A stable tag also needs the download example in `docs/install.md` at its version (`scripts/doc-version.sh check`). Then it runs `lint` and `test`. `make dist` checks that the release key signed `SHA256SUMS`. A tag with `-PRE` becomes a GitHub prerelease; `make publish` then skips COPR, PPA and Homebrew, and the changelogs and release notes of the next stable release start at the previous stable one. The release notes name the Go toolchain. `make test-release` runs the tests of these checks.

Individual targets are also available:

| Target    | Description                                    |
|-----------|------------------------------------------------|
| `tag`     | Create and push a signed git tag               |
| `dist`    | Build the release archives with the pinned Go toolchain + `SHA256SUMS` and sign it (needs a clean checkout of the tag) |
| `release` | Tag + checks + `lint` + `test` + `dist` + create the GitHub release with the archives (uploads them to an existing release that has none) |
| `srpm`    | Build a source RPM                             |
| `copr`    | Build SRPM + submit to Fedora COPR for every supported Fedora and arch (x86_64, aarch64) that lacks this version |
| `ppa`     | Tag + build and upload to Ubuntu PPA (skips Ubuntu releases that already have this version) |
| `image`   | Build the container image of the `dist` archives (podman or docker) |
| `push-image` | Rebuild the image from the signed assets of the GitHub release and push it to `ghcr.io/al-maisan/gmt-mail` |
| `brew`    | Bump the formula in [al-maisan/homebrew-tap](https://github.com/al-maisan/homebrew-tap) to this version and push it (no-op if it is already there) |
| `publish` | All of the above                               |

For a stable version (`X.Y.Z`), `make release` also checks the ghcr.io login first, builds the container image from the archives (`scripts/image.sh`) before the GitHub release and pushes it after (`make push-image`, `scripts/push-image.sh`). The push rebuilds the image from the linux archives of the GitHub release, checked against its `SHA256SUMS` signed by the release key, pushes it as `vX.Y.Z` with `skopeo` and checks the pushed digest. `latest` moves only if the tag is the highest stable tag on GitHub. This needs podman (or docker), skopeo and a login: `podman login ghcr.io -u al-maisan` with a personal access token (classic) that has `write:packages`. If only the push fails, fix the cause and run `git checkout vX.Y.Z && make push-image VERSION=X.Y.Z`; it can be rerun at any time, and a rerun of `make release` pushes the image if the release exists but the image does not. After the first push, make the package public in its settings on GitHub, once. The image is reproducible: the same archives and commit give the same digest. The CA certificates are pinned by URL and hash in `scripts/image.sh`; update both from time to time. `make test-image` builds and runs a test image.

`make brew` clones the tap, sets the formula's `url` and `sha256` to the tagged source archive, commits and pushes over SSH (`BREW_TAP_URL` overrides the remote; with signed commits that is two key touches). It never moves the formula to an older version. The formula builds from source, so it needs no release binaries. `make test-brew` runs its tests.

`make ppa` asks Launchpad what is already uploaded and does nothing for an Ubuntu release that already has the tagged version, so re-running it without a new tag publishes nothing. It asks with `Cache-Control: no-cache`: Launchpad's API cache can be minutes old and miss a fresh upload. A rebuild reuses the uploaded orig tarball; a first upload builds it with `scripts/src-tarball.sh`, which gives the same tarball for the same tag (fixed file order, owner, modes and times, no gzip timestamp), so it matches an upload it missed. If only PPA packaging files changed, force a rebuild with the next PPA revision:

    $ make ppa PPA_REV=2

`make test-ppa` runs the tests for that upload detection.

The PPA source comes from the tag, but `debian/`, `deb-layout.sh` and `install-docs.sh` come from the checkout. So a tag that lacks a file in the `install-docs.sh` list can't be built for the PPA (`install-docs: no '…'`). v0.10.0 is such a tag, so new Ubuntu releases and `PPA_REV` rebuilds work again from the next tag on.

`make copr` targets every Fedora release that is both released (final images on fedoraproject.org) and still maintained (Bodhi), on x86_64 and aarch64 (`ARCHES` in `scripts/copr-build.sh`), so new Fedora releases are picked up automatically and enabled in the COPR project as needed. It builds only for the releases that don't have the tagged version yet and does nothing otherwise. Force a rebuild with `make copr COPR_FORCE=1`, or pick releases with `make copr COPR_CHROOTS="fedora-44-x86_64"`. `make test-copr` runs the tests for this logic.

The man page `gmt-mail.1` carries `@VERSION@`/`@DATE@` placeholders in its `.TH` header. The RPM spec and the Debian rules fill them in from the package version and its changelog date, so every package (`make copr`, `make ppa`, `make publish`) ships a man page with its own version and release date, without a manual bump. `make man` renders it to `~/.cache/gmt-mail/man/gmt-mail.1` for viewing (`man -l ~/.cache/gmt-mail/man/gmt-mail.1`); `make test-man` checks the substitution.

`make dist` cross-compiles the binaries with the official Go toolchain pinned in the Makefile (`RELEASE_GO`; bump it with Go patch releases), the same `-ldflags` as `make build`, plus `-trimpath`, and packs each with LICENSE, the docs (`scripts/install-docs.sh`) and the man page into `~/.cache/gmt-mail/dist/<version>/`. A stable version (`X.Y.Z`) also gets a `.deb` per linux target (`scripts/deb.sh`, which needs `dpkg-deb`; Fedora: `sudo dnf install dpkg`), with the same files as the PPA package: both use `scripts/deb-layout.sh`. The archives are reproducible: the same tag built with the same Go version gives the same `SHA256SUMS` (the Go version is embedded in the binaries; for the `.deb` files, given the same `dpkg-deb` and xz versions). Signing `SHA256SUMS` uses `SIGNING_KEY` (default: git's `user.signingkey`). `make test-dist` runs its tests. `make dist DIST_TARGETS="linux/amd64"` limits the targets for a local build; a `DIST_TARGETS` environment variable is ignored, so a release always ships every target.
