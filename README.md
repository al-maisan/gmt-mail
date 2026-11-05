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

Drop `-dry-run` to send. The demo's files are in [`docs/demo/`](docs/demo/).

## Why gmt-mail?

- **One binary, no setup.** No Python environment, mail client or web service. It runs the same on a laptop, a server or in CI.
- **Safe to run.** `-validate` checks the config, the addresses, the attachments and every placeholder before anything is sent. `-dry-run` prints every message. Unknown config keys and leftover `%KEY%` placeholders are errors, not silent mistakes.
- **Secure by default.** TLS is mandatory (STARTTLS, or implicit TLS on port 465), with no plaintext fallback. The password comes from the environment and is never logged.
- **Personal, not a newsletter.** Each person gets a normal email from your own address, with per-recipient Cc and attachments. Replies come back to you.
- **Robust sending.** Retries, reconnects when the server drops the connection, an optional delay between messages, and a summary with a clear exit code.

It is meant for tens to a few hundred personal messages, not for newsletters. See [how it compares](docs/comparison.md) with mail-merge scripts, Thunderbird and Mailchimp.

## Install

    $ brew install al-maisan/tap/gmt-mail                    # macOS, Linux
    $ sudo add-apt-repository ppa:al-maisan/gmt-mail && sudo apt install gmt-mail   # Ubuntu
    $ sudo dnf copr enable al-maisan/gmt-mail && sudo dnf install gmt-mail          # Fedora
    $ go install github.com/al-maisan/gmt-mail@latest        # Go 1.26+

Signed prebuilt binaries (Linux, macOS, Windows), `.deb` packages, a container image and building from source: see [docs/install.md](docs/install.md).

## Quick start

Create sample files to edit:

    $ gmt-mail -sample-config > config.toml
    $ gmt-mail -sample-template > template.eml

Keep the recipients in a spreadsheet instead? Export it as CSV and pass `-recipients-path people.csv` (`gmt-mail -sample-csv` prints an example); see [Recipients from a CSV file](docs/usage.md#recipients-from-a-csv-file).

Put your SMTP account in a `.env` file in the same directory (Gmail needs an [App Password](https://myaccount.google.com/apppasswords); [`.env.example`](.env.example) has other providers):

    SMTP_HOST=smtp.gmail.com
    SMTP_PORT=587
    SENDER_EMAIL=you@gmail.com
    SENDER_PASSWORD=your-app-password

Check, preview, then send:

    $ gmt-mail -validate -config-path config.toml -template-path template.eml
    $ gmt-mail -dry-run -config-path config.toml -template-path template.eml
    $ gmt-mail -config-path config.toml -template-path template.eml

## Documentation

- [Usage](docs/usage.md): SMTP setup, config file, template variables, dry run, exit codes, CLI reference. Also `man gmt-mail`.
- [Installing](docs/install.md), including how to verify a download.
- [Contributing](CONTRIBUTING.md): development and releasing.
- [Security](SECURITY.md): report a vulnerability privately.

## License

[GNU General Public License v3](LICENSE)
