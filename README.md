<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo.svg">
    <img src="assets/logo.svg" alt="gmt-mail logo" width="200">
  </picture>
</p>

# gmt-mail

Send the same email to many people, each copy personalized, straight from the terminal. You keep the recipients and their details in a TOML file and the text in a template. `gmt-mail` fills in each person's values and sends every copy from your own SMTP account (Gmail, Fastmail, your company server, …) over TLS.

![gmt-mail demo: validate, dry run, then send](docs/demo/demo.gif)

## Example

`config.toml`:

```toml
[general]
from = '"Frodo Baggins" <frodo@shire.example>'
subject = "Your seat at the %EVENT%, %FN%"

[[recipients]]
email = "sam@shire.example"
first = "Samwise"
data = { EVENT = "Long-expected Party", SEAT = "12" }

[[recipients]]
email = "merry@shire.example"
first = "Meriadoc"
data = { EVENT = "Long-expected Party", SEAT = "7" }
```

`template.eml`:

```
Hi %FN%,

your seat at the %EVENT% is number %SEAT%.

Cheers,
Frodo
```

`gmt-mail -dry-run -config-path config.toml -template-path template.eml` shows what each person gets:

```
--
"Samwise" <sam@shire.example>
Subject: Your seat at the Long-expected Party, Samwise
Hi Samwise,

your seat at the Long-expected Party is number 12.

Cheers,
Frodo
```

Drop `-dry-run` to send. The files for this example and the demo are in [`docs/demo/`](docs/demo/).

## Why gmt-mail?

- **One binary, no setup.** No Python environment, mail client or web service. It runs the same on a laptop, a server or in CI.
- **Safe to run.** `-validate` checks the config, the addresses, the attachments and every placeholder before anything is sent. `-dry-run` prints every message. Unknown config keys and leftover placeholders are errors, not silent mistakes.
- **Secure by default.** TLS is mandatory (STARTTLS, or implicit TLS on port 465), with no plaintext fallback. The password comes from the environment and is never logged.
- **Personal, not a newsletter.** Each person gets a normal email from your own address, with per-recipient Cc and attachments. Replies come back to you.
- **Robust sending.** Retries, reconnects when the server drops the connection, an optional delay between messages, and a summary with a clear exit code.

Compared with the usual alternatives:

| | gmt-mail | Python mail-merge script | Thunderbird mail merge | Mailchimp & co. |
|---|---|---|---|---|
| Install | one binary | Python + code you maintain | Thunderbird + add-on | web account |
| Scriptable / CI | yes | yes | no | via API |
| Checks before sending | `-validate`, `-dry-run` | what you write | preview | preview |
| Sent from | your SMTP account | your SMTP account | your mail account | their servers |
| Your list stays with you | yes | yes | yes | no |

`gmt-mail` is meant for tens to a few hundred personal messages. For large newsletters that need unsubscribe handling and bounce tracking, use an email service provider.

## Installation

### Prebuilt binaries (Linux, macOS, Windows)

Every [release](https://github.com/al-maisan/gmt-mail/releases) has archives for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 (zip). Each holds the `gmt-mail` binary, the man page, README and LICENSE. `SHA256SUMS` is signed with the [maintainer's key](https://github.com/al-maisan.gpg). Example for Linux amd64 (set `V` to the latest release):

    $ V=0.9.1 A=gmt-mail-$V-linux-amd64
    $ U=https://github.com/al-maisan/gmt-mail/releases/download/v$V
    $ curl -LO $U/$A.tar.gz -LO $U/SHA256SUMS -LO $U/SHA256SUMS.asc
    $ curl -s https://github.com/al-maisan.gpg | gpg --import
    $ gpg --verify SHA256SUMS.asc SHA256SUMS
    $ sha256sum --ignore-missing -c SHA256SUMS
    $ tar xzf $A.tar.gz
    $ sudo install -m 0755 $A/gmt-mail /usr/local/bin/
    $ sudo install -Dm 0644 $A/gmt-mail.1 /usr/local/share/man/man1/gmt-mail.1

On macOS use `shasum -a 256 --ignore-missing -c SHA256SUMS`. macOS may block the unsigned binary on first run; allow it with `xattr -d com.apple.quarantine $A/gmt-mail`.

### macOS and Linux ([Homebrew](https://github.com/al-maisan/homebrew-tap))

    $ brew install al-maisan/tap/gmt-mail

### Ubuntu ([PPA](https://launchpad.net/~al-maisan/+archive/ubuntu/gmt-mail))

    $ sudo add-apt-repository ppa:al-maisan/gmt-mail
    $ sudo apt update
    $ sudo apt install gmt-mail

### Fedora ([COPR](https://copr.fedorainfracloud.org/coprs/al-maisan/gmt-mail/))

    $ sudo dnf copr enable al-maisan/gmt-mail
    $ sudo dnf install gmt-mail

### With Go

Requires [Go](https://go.dev/) 1.26 or later.

    $ go install github.com/al-maisan/gmt-mail@latest

This installs `gmt-mail` to `$(go env GOPATH)/bin`. `-version` shows the module version.

### From source

Requires [Go](https://go.dev/) 1.26 or later.

    $ git clone https://github.com/al-maisan/gmt-mail.git
    $ cd gmt-mail
    $ make build
    $ ./gmt-mail -h

The Makefile embeds the version, git commit hash, and build date into the binary. CI runs `make vet`, golangci-lint, `make test`, `make test-pkg` (the tests of the packaging and release scripts) and `make build` on every pull request. Targets: `all`, `build`, `test`, `vet`, `lint`, `fmt`, `install`, `vendor`, `clean`, `srpm`, `copr`, `ppa`, `test-ppa`, `test-copr`, `man`, `test-man`, `tag`, `dist`, `test-dist`, `release`, `brew`, `test-brew`, `test-pkg`, `publish`.

## Quick start

Generate sample files, configure SMTP credentials, preview, then send:

    $ ./gmt-mail -sample-config > config.toml
    $ ./gmt-mail -sample-template > template.eml
    $ cp .env.example .env    # edit .env with your SMTP credentials
    $ ./gmt-mail -validate -config-path config.toml -template-path template.eml
    $ ./gmt-mail -dry-run -config-path config.toml -template-path template.eml
    $ ./gmt-mail -config-path config.toml -template-path template.eml

## SMTP setup

SMTP credentials are read from environment variables. If a `.env` file exists in the working directory, it is loaded automatically.

Copy `.env.example` to `.env` and fill in:

    SMTP_HOST=smtp.gmail.com
    SMTP_PORT=587
    SENDER_EMAIL=your-email@gmail.com
    SENDER_PASSWORD=your-app-password

`SENDER_EMAIL` / `SENDER_PASSWORD` are used for SMTP authentication. The `From` header in the config file controls what recipients see as the sender -- these can differ if your mail server allows it.

For Gmail you must use an [App Password](https://myaccount.google.com/apppasswords) (requires 2-Step Verification). Other providers (Outlook, Yahoo, etc.) are documented in `.env.example`.

TLS is enforced -- credentials are never sent in plaintext. Port 465 uses implicit TLS (SMTPS); any other port (e.g. 587) requires STARTTLS.

## Configuration file

The config file uses [TOML](https://toml.io/) format with a `[general]` section and one or more `[[recipients]]` entries. Unknown keys are rejected and all addresses are checked when the file is parsed, so `-validate` catches typos before anything is sent.

### `[general]` section

| Key           | Required | Description                                  |
|---------------|----------|----------------------------------------------|
| `from`        | yes      | Sender name and address for the email header  |
| `subject`     | yes      | Email subject line (supports template vars)   |
| `cc`          | no       | List of CC addresses                          |
| `reply_to`    | no       | Reply-To address                              |
| `attachments` | no       | List of file paths to attach                  |

### `[[recipients]]` entries

Each recipient is a TOML table with these fields:

| Key                  | Required | Description                                  |
|----------------------|----------|----------------------------------------------|
| `email`              | yes      | Recipient email address                       |
| `first`              | yes      | First name                                    |
| `last`               | no       | Last name                                     |
| `data`               | no       | Custom key-value pairs for template variables  |
| `cc`                 | no       | Replace global Cc for this recipient           |
| `cc_extra`           | no       | Append to global Cc for this recipient         |
| `attachments`        | no       | Replace global attachments for this recipient  |
| `attachments_extra`  | no       | Append to global attachments for this recipient|

Each `email` must be unique. A recipient may set `cc` or `cc_extra`, not both (same for `attachments` / `attachments_extra`). Attachments must be readable regular files.

Example:

```toml
[general]
from = '"Frodo Baggins" <frodo@shire.org>'
subject = "Hello %FN%!"
cc = ["gandalf@shire.org"]
reply_to = '"Frodo" <frodo@shire.org>'
attachments = ["/path/to/map.pdf"]

[[recipients]]
email = "sam@shire.org"
first = "Samwise"
last = "Gamgee"
data = { ROLE = "Gardener" }

[[recipients]]
email = "pippin@shire.org"
first = "Peregrin"
last = "Took"
data = { ROLE = "Knight" }
cc = ["merry@shire.org"]
```

## Template variables

Templates support these placeholders (in both subject and body). Custom keys must start with a letter and contain only letters, digits and `_`; they are matched in **uppercase** -- use `%ORG%` not `%org%`. Because keys are case-folded, two data keys that differ only in case (e.g. `url` and `URL`) are rejected as a collision, and a custom key may not reuse a reserved name (`EA`, `FN`, `LN`).

| Placeholder | Value                                    |
|-------------|------------------------------------------|
| `%FN%`      | First name                               |
| `%LN%`      | Last name                                |
| `%EA%`      | Email address                            |
| `%KEY%`     | Any custom key from recipient data       |

Example template:

    Dear %FN% %LN%,

    How are things going at %ORG%?

    Best regards

## Dry run

Use `-dry-run` to preview all emails without sending. The output includes Cc and attachment information when present:

    $ ./gmt-mail -dry-run -config-path config.toml -template-path template.eml
    --
    "Samwise Gamgee" <sam@shire.org>
    Cc: gandalf@shire.org
    Subject: Hello Samwise!
    Attachments: /path/to/map.pdf
    Dear Samwise Gamgee,

    How are things going, Gardener?

    Best regards
    --
    "Peregrin Took" <pippin@shire.org>
    Cc: merry@shire.org
    Subject: Hello Peregrin!
    Attachments: /path/to/map.pdf
    Dear Peregrin Took,

    How are things going, Knight?

    Best regards

## Exit codes

| Code | Meaning                          |
|------|----------------------------------|
| 0    | Success                          |
| 1    | Usage error (missing or invalid flags) |
| 2    | Config or template file error    |
| 3    | SMTP connection error            |
| 4    | One or more emails failed to send|

## CLI reference

    $ ./gmt-mail -h

      -config-path string
            path to the config file
      -delay duration
            delay between emails, e.g., 1s, 500ms (default 0s)
      -dry-run
            show what would be done but execute no action
      -retries int
            max retry attempts per failed send (default 1)
      -retry-delay duration
            backoff between retries (default 2s)
      -sample-config
            output sample configuration to stdout
      -sample-template
            output sample template to stdout
      -template-path string
            path to the template file
      -timeout duration
            SMTP connect/send timeout, must be > 0 (covers the full attachment upload) (default 30s)
      -validate
            validate config and template without sending
      -version
            print version and exit

Transient send failures are retried automatically (controlled by `-retries` and `-retry-delay`). If the SMTP connection is dropped mid-batch (e.g. a server idle-timeout or per-connection message cap), it is re-established before the next attempt so the rest of the batch is not lost. Large attachments over slow links may need a higher `-timeout`. Progress is shown as `[1/N]` for each message.

## Releasing a new version

The version is derived from the latest git tag — there is no `VERSION` to edit.
Cut a release by creating and pushing a signed tag, then publish everywhere:

    $ git tag -s v0.7.0 -m "v0.7.0"
    $ git push origin v0.7.0
    $ make publish

`make publish` creates the GitHub release with prebuilt archives, submits to Fedora COPR, uploads to Ubuntu PPA and bumps the Homebrew formula, all using the tagged version. To cut it in one shot without tagging first, override: `make publish VERSION=0.7.0` creates and pushes the tag for you.

> **Note:** both package changelogs (the RPM `%changelog` and the Debian `debian/changelog`) are generated from `git log` between **GitHub releases**, so a version appears in them only once a GitHub *release* exists for its tag. `make publish` (and `make release`) create the release *before* building packages, so a full `make publish` yields complete changelogs — but building packages standalone (`make srpm`/`make copr`/`make ppa`) before the release exists will omit that version.

Individual targets are also available:

| Target    | Description                                    |
|-----------|------------------------------------------------|
| `tag`     | Create and push a signed git tag               |
| `dist`    | Build the release archives + `SHA256SUMS` and sign it (needs a clean checkout of the tag) |
| `release` | Tag + `dist` + create the GitHub release with the archives (uploads them to an existing release that has none) |
| `srpm`    | Build a source RPM                             |
| `copr`    | Build SRPM + submit to Fedora COPR for every supported Fedora that lacks this version |
| `ppa`     | Tag + build and upload to Ubuntu PPA (skips Ubuntu releases that already have this version) |
| `brew`    | Bump the formula in [al-maisan/homebrew-tap](https://github.com/al-maisan/homebrew-tap) to this version and push it (no-op if it is already there) |
| `publish` | All of the above                               |

`make brew` clones the tap, sets the formula's `url` and `sha256` to the tagged source archive, commits and pushes over SSH (`BREW_TAP_URL` overrides the remote; with signed commits that is two key touches). It never moves the formula to an older version. The formula builds from source, so it needs no release binaries. `make test-brew` runs its tests.

`make ppa` asks Launchpad what is already uploaded and does nothing for an Ubuntu release that already has the tagged version, so re-running it without a new tag publishes nothing. If only PPA packaging files changed, force a rebuild with the next PPA revision:

    $ make ppa PPA_REV=2

`make test-ppa` runs the tests for that upload detection.

`make copr` targets every Fedora release that is both released (final images on fedoraproject.org) and still maintained (Bodhi), so new Fedora releases are picked up automatically and enabled in the COPR project as needed. It builds only for the releases that don't have the tagged version yet and does nothing otherwise. Force a rebuild with `make copr COPR_FORCE=1`, or pick releases with `make copr COPR_CHROOTS="fedora-44-x86_64"`. `make test-copr` runs the tests for this logic.

The man page `gmt-mail.1` carries `@VERSION@`/`@DATE@` placeholders in its `.TH` header. The RPM spec and the Debian rules fill them in from the package version and its changelog date, so every package (`make copr`, `make ppa`, `make publish`) ships a man page with its own version and release date, without a manual bump. `make man` renders it to `~/.cache/gmt-mail/man/gmt-mail.1` for viewing (`man -l ~/.cache/gmt-mail/man/gmt-mail.1`); `make test-man` checks the substitution.

`make dist` cross-compiles the binaries (same `-ldflags` as `make build`, plus `-trimpath`) and packs each with LICENSE, README and the man page into `~/.cache/gmt-mail/dist/<version>/`. The archives are reproducible: the same tag built with the same Go version gives the same `SHA256SUMS` (the Go version is embedded in the binaries). Signing `SHA256SUMS` uses `SIGNING_KEY` (default: git's `user.signingkey`). `make test-dist` runs its tests. `make dist DIST_TARGETS="linux/amd64"` limits the targets for a local build; a `DIST_TARGETS` environment variable is ignored, so a release always ships every target.

## License

[GNU General Public License v3](LICENSE)
