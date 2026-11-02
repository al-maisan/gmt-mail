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

Every [release](https://github.com/al-maisan/gmt-mail/releases) has archives for linux/amd64, linux/arm64, linux/armv6 (a 32-bit OS on any Raspberry Pi), darwin/amd64, darwin/arm64 and windows/amd64 (zip). Each holds the `gmt-mail` binary, the man page, README and LICENSE. `SHA256SUMS` is signed with the [maintainer's key](https://github.com/al-maisan.gpg). Example for Linux amd64 (set `V` to the latest release):

    $ V=0.10.0 A=gmt-mail-$V-linux-amd64
    $ U=https://github.com/al-maisan/gmt-mail/releases/download/v$V
    $ curl -LO $U/$A.tar.gz -LO $U/SHA256SUMS -LO $U/SHA256SUMS.asc
    $ curl -s https://github.com/al-maisan.gpg | gpg --import
    $ gpg --verify SHA256SUMS.asc SHA256SUMS
    $ sha256sum --ignore-missing -c SHA256SUMS
    $ tar xzf $A.tar.gz
    $ sudo install -m 0755 $A/gmt-mail /usr/local/bin/
    $ sudo install -Dm 0644 $A/gmt-mail.1 /usr/local/share/man/man1/gmt-mail.1

See [Verify a download](#verify-a-download) for what the `gpg` and `sha256sum` steps must show. macOS may block the unsigned binary on first run; allow it with `xattr -d com.apple.quarantine $A/gmt-mail`.

### Verify a download

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

### Debian, Raspberry Pi OS, other Ubuntu releases (`.deb`)

Releases after v0.10.0 also have `.deb` packages for amd64, arm64 and armhf (32-bit Raspberry Pi OS). Download the one for your system with `SHA256SUMS` and `SHA256SUMS.asc` and check them as above, then install it (example for arm64):

    $ sudo apt install ./gmt-mail_${V}-1_arm64.deb

It installs the same files as the PPA package. Updates are by hand: install the next release's `.deb` the same way.

### macOS and Linux ([Homebrew](https://github.com/al-maisan/homebrew-tap))

    $ brew install al-maisan/tap/gmt-mail

### Ubuntu ([PPA](https://launchpad.net/~al-maisan/+archive/ubuntu/gmt-mail))

    $ sudo add-apt-repository ppa:al-maisan/gmt-mail
    $ sudo apt update
    $ sudo apt install gmt-mail

### Fedora ([COPR](https://copr.fedorainfracloud.org/coprs/al-maisan/gmt-mail/))

    $ sudo dnf copr enable al-maisan/gmt-mail
    $ sudo dnf install gmt-mail

### Container image

`ghcr.io/al-maisan/gmt-mail` for linux/amd64, linux/arm64 and linux/arm/v6, built from the release archives (releases after v0.10.0). Mount the directory with your config, template, attachments and `.env` at `/work` and run as your own user, so gmt-mail can read the files:

    $ podman run --rm --userns=keep-id --user "$(id -u):$(id -g)" -v "$PWD:/work:Z" \
        ghcr.io/al-maisan/gmt-mail -dry-run -config-path config.toml -template-path template.eml

With docker, drop `--userns=keep-id` and `:Z`. Paths in the config (e.g. attachments) must be inside `/work`.

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

The Makefile embeds the version, git commit hash, and build date into the binary.

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and releasing, and [SECURITY.md](SECURITY.md) to report a vulnerability.

## License

[GNU General Public License v3](LICENSE)
