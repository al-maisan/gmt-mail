# Using gmt-mail

The man page (`man gmt-mail`) has the same reference.

- [SMTP setup](#smtp-setup)
- [Configuration file](#configuration-file)
- [Template variables](#template-variables)
- [Dry run](#dry-run)
- [Sending](#sending)
- [Exit codes](#exit-codes)
- [CLI reference](#cli-reference)

## SMTP setup

SMTP credentials are read from environment variables. If a `.env` file exists in the working directory, it is loaded automatically:

    SMTP_HOST=smtp.gmail.com
    SMTP_PORT=587
    SENDER_EMAIL=your-email@gmail.com
    SENDER_PASSWORD=your-app-password

Variables already set in the environment win over `.env`. So run gmt-mail only in a directory whose `.env` you trust: with `SENDER_PASSWORD` exported in your shell, someone else's `.env` setting `SMTP_HOST` would get your password.

`SENDER_EMAIL` and `SENDER_PASSWORD` are used for SMTP authentication. The `from` key in the config file sets what recipients see as the sender; the two can differ if your mail server allows it.

For Gmail you must use an [App Password](https://myaccount.google.com/apppasswords) (needs 2-Step Verification). [`.env.example`](../.env.example) also has settings for Outlook and Yahoo.

TLS is enforced, so credentials are never sent in plaintext. Port 465 uses implicit TLS (SMTPS); any other port (e.g. 587) requires STARTTLS. Login uses SMTP AUTH PLAIN only: a server that doesn't offer it fails the connection (exit 3).

## Configuration file

The config file uses [TOML](https://toml.io/) with a `[general]` section and one or more `[[recipients]]` entries, or the recipients come from a [CSV file](#recipients-from-a-csv-file). Unknown keys are rejected and all addresses are checked when the file is parsed, so `-validate` catches typos before anything is sent.

### `[general]`

| Key           | Required | Description                                 |
|---------------|----------|---------------------------------------------|
| `from`        | yes      | Sender name and address for the email header |
| `subject`     | yes      | Subject line (supports template variables)   |
| `cc`          | no       | List of Cc addresses                         |
| `reply_to`    | no       | Reply-To address                             |
| `attachments` | no       | List of file paths to attach                 |

### `[[recipients]]`

| Key                 | Required | Description                                    |
|---------------------|----------|------------------------------------------------|
| `email`             | yes      | Recipient address                              |
| `first`             | yes      | First name (not blank)                         |
| `last`              | no       | Last name                                      |
| `data`              | no       | Custom key-value pairs for template variables  |
| `cc`                | no       | Replaces the global Cc for this recipient      |
| `cc_extra`          | no       | Is added to the global Cc for this recipient   |
| `attachments`       | no       | Replaces the global attachments for this recipient |
| `attachments_extra` | no       | Is added to the global attachments for this recipient |

Each `email` must be unique. A recipient's Cc never lists the same address twice (compared case-insensitively) or the recipient's own address. If the server rejects any one address of a message (To or a Cc, e.g. a stale `cc_extra` address), that message goes to nobody: it counts as failed (exit 4). A recipient may set `cc` or `cc_extra`, not both (same for `attachments` and `attachments_extra`). Attachments must be readable regular files.

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

### Recipients from a CSV file

`-recipients-path people.csv` reads the recipients from a CSV file, e.g. one exported from a spreadsheet. The config file then keeps only `[general]`: `[[recipients]]` is an error. `gmt-mail -sample-csv` prints an example.

The file is UTF-8 (a leading BOM is ignored), comma-separated, with quoted fields as in RFC 4180. The first row names the columns, in any order and case:

- `email`, `first` (both required) and `last`: as in `[[recipients]]`.
- `cc`, `cc_extra`, `attachments`, `attachments_extra`: several values separated by `;`, e.g. `a@x.org; b@x.org`. An empty cell means not set. The attachment columns are rejected unless you pass `-csv-attachments`: a CSV from someone else could otherwise attach any of your files (e.g. `~/.ssh/id_rsa`) to a mail for any address. Attachments in `[general]` are not affected.
- Every other column is a custom key for `%KEY%` (see below), e.g. a column `role` fills `%ROLE%`.

Each row gets the same checks as a `[[recipients]]` entry. A duplicate column name, a row with the wrong number of fields, an empty file or a file without rows is an error. Errors name the file and line, e.g. `people.csv:3: recipient "sam@shire.org": 'first' must not be blank`. Check a CSV from someone else with `-dry-run` before sending: it shows every recipient, Cc and attachment.

The recipients of the example above as CSV:

```csv
email,first,last,role,cc
sam@shire.org,Samwise,Gamgee,Gardener,
pippin@shire.org,Peregrin,Took,Knight,merry@shire.org
```

    $ gmt-mail -dry-run -config-path general.toml -recipients-path people.csv -template-path template.eml

## Template variables

Placeholders work in the subject and the body:

| Placeholder | Value                              |
|-------------|------------------------------------|
| `%FN%`      | First name                         |
| `%LN%`      | Last name                          |
| `%EA%`      | Email address                      |
| `%KEY%`     | Any custom key from `data`         |

Custom keys start with a letter and hold only letters, digits and `_`. They are matched in **uppercase**: use `%ROLE%`, not `%role%`. So two keys that differ only in case (e.g. `url` and `URL`) are rejected, and a custom key may not be `EA`, `FN` or `LN`. A placeholder that no value fills is an error, and so is a subject or body that is blank after substitution (e.g. `subject = "%LN%"` for a recipient without `last`).

Example template:

    Dear %FN% %LN%,

    How are things going, %ROLE%?

    Best regards

## Dry run

`-dry-run` prints every message without sending, with Cc and attachments when present. Like `-validate`, it first checks that every attachment file exists and is readable. The example above gives:

    $ gmt-mail -dry-run -config-path config.toml -template-path template.eml
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

## Sending

Progress is shown as `[1/N]` for each message. Temporary send failures are retried (`-retries`, `-retry-delay`); a permanent (5xx) rejection is not. If the server drops the connection mid-batch (e.g. an idle timeout or a per-connection message cap), it is re-established before the next attempt, so the rest of the batch is not lost. `-delay` waits between messages. Large attachments over slow links may need a higher `-timeout`.

## Exit codes

| Code | Meaning                                 |
|------|-----------------------------------------|
| 0    | Success                                 |
| 1    | Usage error (missing or invalid flags)  |
| 2    | Config or template file error           |
| 3    | SMTP connection error                   |
| 4    | One or more emails failed to send       |

## CLI reference

    $ gmt-mail -h

      -config-path string
            path to the config file
      -csv-attachments
            allow the attachments and attachments_extra columns in the -recipients-path CSV
      -delay duration
            delay between emails (e.g., 1s, 500ms)
      -dry-run
            show what would be done but execute no action
      -recipients-path string
            path to a CSV file with the recipients (the config file then has no [[recipients]])
      -retries int
            max retry attempts per failed send (default 1)
      -retry-delay duration
            backoff between retries (default 2s)
      -sample-config
            output sample configuration to stdout
      -sample-csv
            output sample recipients CSV to stdout
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
