# Installing gmt-mail

- [Homebrew (macOS, Linux)](#homebrew-macos-linux)
- [Ubuntu PPA](#ubuntu-ppa)
- [Fedora COPR](#fedora-copr)
- [Prebuilt binaries (Linux, macOS, Windows)](#prebuilt-binaries-linux-macos-windows)
- [Verify a download](#verify-a-download)
- [Debian, Raspberry Pi OS, other Ubuntu releases (`.deb`)](#debian-raspberry-pi-os-other-ubuntu-releases-deb)
- [Container image](#container-image)
- [With Go](#with-go)
- [From source](#from-source)

## Homebrew (macOS, Linux)

From the [al-maisan/homebrew-tap](https://github.com/al-maisan/homebrew-tap):

    $ brew install al-maisan/tap/gmt-mail

## Ubuntu PPA

From the [PPA](https://launchpad.net/~al-maisan/+archive/ubuntu/gmt-mail):

    $ sudo add-apt-repository ppa:al-maisan/gmt-mail
    $ sudo apt update
    $ sudo apt install gmt-mail

## Fedora COPR

From [COPR](https://copr.fedorainfracloud.org/coprs/al-maisan/gmt-mail/), x86_64 and aarch64:

    $ sudo dnf copr enable al-maisan/gmt-mail
    $ sudo dnf install gmt-mail

## Prebuilt binaries (Linux, macOS, Windows)

[Releases](https://github.com/al-maisan/gmt-mail/releases) from v0.10.0 on have archives for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 (zip). Releases after v0.10.0 also have linux/armv6 (a 32-bit OS on any Raspberry Pi). Each archive holds the `gmt-mail` binary, the man page, LICENSE and these docs. `SHA256SUMS` is signed with the [maintainer's key](https://github.com/al-maisan.gpg).

Example for Linux amd64 (set `V` to the latest release):

    $ V=0.11.1 A=gmt-mail-$V-linux-amd64
    $ U=https://github.com/al-maisan/gmt-mail/releases/download/v$V
    $ curl -LO $U/$A.tar.gz -LO $U/SHA256SUMS -LO $U/SHA256SUMS.asc
    $ curl -s https://github.com/al-maisan.gpg | gpg --import
    $ gpg --verify SHA256SUMS.asc SHA256SUMS
    $ sha256sum --ignore-missing -c SHA256SUMS
    $ tar xzf $A.tar.gz
    $ sudo install -m 0755 $A/gmt-mail /usr/local/bin/
    $ sudo install -Dm 0644 $A/gmt-mail.1 /usr/local/share/man/man1/gmt-mail.1

[Verify a download](#verify-a-download) shows what the `gpg` and `sha256sum` steps must print. macOS may block the unsigned binary on first run; allow it with `xattr -d com.apple.quarantine $A/gmt-mail`.

## Verify a download

In the directory with the download, `SHA256SUMS` and `SHA256SUMS.asc`:

1. Import the release key:

       $ curl -s https://github.com/al-maisan.gpg | gpg --import

   You should see `key 1E0A288397AE739E: public key "Muharem Hrnjadovic …" imported` (or `not changed`). Lines such as `signatures not checked due to missing keys` and `no ultimately trusted keys found` are normal.

2. Check the signature of the checksums:

       $ gpg --verify SHA256SUMS.asc SHA256SUMS

   You should see `Good signature from "Muharem Hrnjadovic …"` and `using RSA key 753B6ECF2B458FF3D19D568C1E0A288397AE739E`. The warning that the key is not certified is expected; any other key, or `BAD signature`, means stop.

3. Check the download against them:

       $ sha256sum --ignore-missing -c SHA256SUMS

   You should see your file's name followed by `OK`, e.g. `gmt-mail-<version>-linux-amd64.tar.gz: OK`. On macOS use `shasum -a 256 --ignore-missing -c SHA256SUMS`.

For a script, this checks the same and fails unless the release key signed:

    $ gpg --status-fd 1 --verify SHA256SUMS.asc SHA256SUMS 2>/dev/null | grep -q '^\[GNUPG:\] VALIDSIG .* 753B6ECF2B458FF3D19D568C1E0A288397AE739E$' && sha256sum --ignore-missing -c SHA256SUMS

## Debian, Raspberry Pi OS, other Ubuntu releases (`.deb`)

Releases after v0.10.0 also have `.deb` packages for amd64, arm64 and armhf (32-bit Raspberry Pi OS). Download the one for your system with `SHA256SUMS` and `SHA256SUMS.asc`, [verify](#verify-a-download) them, then install it (example for arm64):

    $ sudo apt install ./gmt-mail_${V}-1_arm64.deb

It installs the same files as the PPA package. Updates are by hand: install the next release's `.deb` the same way.

## Container image

`ghcr.io/al-maisan/gmt-mail` for linux/amd64, linux/arm64 and linux/arm/v6, built from the release archives (releases after v0.10.0). Mount the directory with your config, template, attachments and `.env` at `/work` and run as your own user, so gmt-mail can read the files:

    $ podman run --rm --userns=keep-id --user "$(id -u):$(id -g)" -v "$PWD:/work:Z" \
        ghcr.io/al-maisan/gmt-mail -dry-run -config-path config.toml -template-path template.eml

With docker, drop `--userns=keep-id` and `:Z`. Paths in the config (e.g. attachments) must be inside `/work`.

## With Go

Requires [Go](https://go.dev/) 1.26 or later.

    $ go install github.com/al-maisan/gmt-mail@latest

This installs `gmt-mail` to `$(go env GOPATH)/bin`. `-version` shows the module version.

## From source

Requires [Go](https://go.dev/) 1.26 or later.

    $ git clone https://github.com/al-maisan/gmt-mail.git
    $ cd gmt-mail
    $ make build
    $ ./gmt-mail -h

The Makefile embeds the version, git commit hash and build date into the binary. See [CONTRIBUTING.md](../CONTRIBUTING.md) for the other targets.
