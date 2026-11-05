// gmt-mail sends emails in bulk based on a template and a config file.
// Copyright (C) 2019-2025  "Muharem Hrnjadovic" <muharem@linux.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package email

import (
	"context"
	"errors"
	"fmt"
	"io"
	netmail "net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/al-maisan/gmt-mail/config"
	mail "github.com/wneessen/go-mail"
	"github.com/wneessen/go-mail/smtp"
)

// Sender abstracts the ability to send an email message.
type Sender interface {
	Send(msg *mail.Msg) error
	// Reconnect re-establishes the underlying connection after it has dropped.
	Reconnect() error
	Close() error
}

// SMTPCredentials holds the SMTP server connection parameters.
type SMTPCredentials struct {
	Host     string
	Port     int
	User     string
	Password string
}

// SendOptions controls rate limiting and retry behavior.
type SendOptions struct {
	Delay      time.Duration // delay between messages
	Retries    int           // max retry attempts per message
	RetryDelay time.Duration // backoff between retries
}

// SendResult holds the outcome of a bulk send operation.
type SendResult struct {
	Sent   int
	Failed int
}

// LoadSMTPCredentials reads SMTP credentials from environment variables.
// Returns an error listing any missing variables.
func LoadSMTPCredentials() (SMTPCredentials, error) {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SENDER_EMAIL")
	password := os.Getenv("SENDER_PASSWORD")

	var missing []string
	if host == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if portStr == "" {
		missing = append(missing, "SMTP_PORT")
	}
	if user == "" {
		missing = append(missing, "SENDER_EMAIL")
	}
	if password == "" {
		missing = append(missing, "SENDER_PASSWORD")
	}
	if len(missing) > 0 {
		return SMTPCredentials{}, fmt.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return SMTPCredentials{}, fmt.Errorf("SMTP_PORT must be a port number (1-65535), got %q", portStr)
	}

	return SMTPCredentials{Host: host, Port: port, User: user, Password: password}, nil
}

// smtpsPort is the SMTPS port, which speaks implicit TLS instead of STARTTLS.
const smtpsPort = 465

// NewSMTPSender creates a connected SMTP sender using the given credentials.
// Port 465 uses implicit TLS; any other port requires STARTTLS. A positive
// timeout bounds each connect and per-message send (covering the full
// DATA/attachment stream); zero uses the go-mail default. The caller must
// call Close when done.
func NewSMTPSender(creds SMTPCredentials, timeout time.Duration) (Sender, error) {
	opts := []mail.Option{
		mail.WithPort(creds.Port),
		mail.WithUsername(creds.User),
		mail.WithPassword(creds.Password),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithTLSPolicy(mail.TLSMandatory),
	}
	if creds.Port == smtpsPort {
		opts = append(opts, mail.WithSSL())
	}
	if timeout > 0 {
		opts = append(opts, mail.WithTimeout(timeout))
	}

	client, err := mail.NewClient(creds.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create SMTP client for %s:%d: %w", creds.Host, creds.Port, err)
	}

	s := &smtpSender{client: client}
	if err := s.dial(); err != nil {
		return nil, fmt.Errorf("failed to connect to %s:%d: %w", creds.Host, creds.Port, err)
	}
	return s, nil
}

// checkAttachment verifies that path is a readable regular file. go-mail only
// reads attachments once DATA has started, so anything else would fail mid-send
// and leave the SMTP session stuck.
// The type is checked before opening because opening a FIFO blocks until a
// writer appears.
func checkAttachment(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// CheckAttachments verifies that every resolved attachment across all messages
// exists and is accessible, so missing files (including per-recipient overrides)
// are reported up front — e.g. during -validate — instead of only mid-send.
func CheckAttachments(msgs []Message) error {
	for _, m := range msgs {
		for _, path := range m.Attachments {
			if err := checkAttachment(path); err != nil {
				return fmt.Errorf("attachment %q for recipient %s: %w", path, m.Address, err)
			}
		}
	}
	return nil
}

// BatchSender holds the per-batch state for delivering a set of messages.
type BatchSender struct {
	w       io.Writer
	sender  Sender
	from    string
	replyTo string
	opts    SendOptions
}

// NewBatchSender creates a BatchSender for delivering a batch of messages.
func NewBatchSender(w io.Writer, sender Sender, cfg config.MailConfig, opts SendOptions) *BatchSender {
	return &BatchSender{w: w, sender: sender, from: cfg.From, replyTo: cfg.ReplyTo, opts: opts}
}

// SendAll delivers all messages, logging progress to w.
// Per-message errors do not stop the batch.
func (sc *BatchSender) SendAll(msgs []Message) SendResult {
	total := len(msgs)
	width := len(fmt.Sprintf("%d", total))
	var result SendResult
	for i, m := range msgs {
		prefix := fmt.Sprintf("[%*d/%*d]", width, i+1, width, total)

		if err := sc.sendOne(m, prefix); err != nil {
			result.Failed++
		} else {
			result.Sent++
		}

		if sc.opts.Delay > 0 && i < total-1 {
			time.Sleep(sc.opts.Delay)
		}
	}
	return result
}

// --- internal ---

// logf writes a formatted message to w, ignoring write errors
// (output is best-effort and must not interrupt the send loop).
func logf(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, format, args...) //nolint:errcheck
}

// smtpSender wraps a go-mail Client to implement Sender. It holds the SMTP
// connection itself so Close can force the socket shut when QUIT fails.
type smtpSender struct {
	client *mail.Client
	conn   *smtp.Client
}

// dial opens a new connection using the client's stored configuration.
func (s *smtpSender) dial() error {
	conn, err := s.client.DialToSMTPClientWithContext(context.Background())
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *smtpSender) Send(msg *mail.Msg) error { return s.client.SendWithSMTPClient(s.conn, msg) }

// Close ends the session with QUIT. go-mail leaves the socket open when QUIT
// fails (the normal case on a dead connection), so close it directly then.
func (s *smtpSender) Close() error {
	if err := s.client.CloseWithSMTPClient(s.conn); err != nil {
		_ = s.conn.Close()
		return err
	}
	return nil
}

// Reconnect closes the (likely dead) connection best-effort and re-dials.
func (s *smtpSender) Reconnect() error {
	_ = s.Close()
	return s.dial()
}

// sendOne prepares and sends a single message, with retries.
func (sc *BatchSender) sendOne(m Message, prefix string) error {
	recipient := fmt.Sprintf("%s <%s>", m.Name, m.Address)

	msg, err := createMessage(sc.from, m.Name, m.Address, m.Cc, sc.replyTo, m.Subject, m.Body)
	if err != nil {
		logf(sc.w, "%s ! %s (failed to create: %v)\n", prefix, recipient, err)
		return err
	}
	if m.HTML != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, m.HTML)
	}

	if err := attachFiles(msg, m.Attachments); err != nil {
		logf(sc.w, "%s ! %s (failed to attach: %v)\n", prefix, recipient, err)
		return err
	}

	if err := sc.sendWithRetry(msg, prefix, recipient); err != nil {
		return err
	}

	logf(sc.w, "%s - %s\n", prefix, recipient)
	if len(m.Cc) > 0 {
		logf(sc.w, "  Cc: %s\n", strings.Join(m.Cc, ", "))
	}
	if len(m.Attachments) > 0 {
		logf(sc.w, "  Attachments: %s\n", strings.Join(m.Attachments, ", "))
	}
	return nil
}

// sendWithRetry attempts to send msg, retrying up to opts.Retries times with
// opts.RetryDelay between attempts. When a failure looks like a dropped
// connection it re-dials before the next attempt, so a server that closes the
// connection mid-batch (idle timeout, per-connection message cap) does not doom
// every remaining recipient.
func (sc *BatchSender) sendWithRetry(msg *mail.Msg, prefix, recipient string) error {
	var err error
	// Always attempt at least once; a negative Retries must never silently
	// skip the send and report success.
	attempts := max(sc.opts.Retries+1, 1)
	for attempt := range attempts {
		if attempt > 0 {
			logf(sc.w, "%s   retrying %s...\n", prefix, recipient)
			if isConnectionError(err) {
				if rcErr := sc.sender.Reconnect(); rcErr != nil {
					logf(sc.w, "%s   reconnect failed: %v\n", prefix, rcErr)
				}
			}
			if sc.opts.RetryDelay > 0 {
				time.Sleep(sc.opts.RetryDelay)
			}
		}
		err = sc.sender.Send(msg)
		if err == nil {
			return nil
		}
		// The server accepted the message but a later step (e.g. RSET) failed:
		// it was delivered, so do not count it as failed or re-send it.
		if deliveredDespiteError(msg, err) {
			return nil
		}
		// The server may have accepted the message and only its final reply
		// was lost: re-sending could deliver it twice. Re-dial so the next
		// recipient gets a working connection.
		if deliveryUnknown(err) {
			logf(sc.w, "%s ! %s (may have been delivered, not retried: %v)\n", prefix, recipient, err)
			if rcErr := sc.sender.Reconnect(); rcErr != nil {
				logf(sc.w, "%s   reconnect failed: %v\n", prefix, rcErr)
			}
			return err
		}
		if isPermanentError(err) {
			break
		}
	}
	logf(sc.w, "%s ! %s (failed to send: %v)\n", prefix, recipient, err)
	return err
}

// isConnectionError reports whether err indicates the SMTP connection is dead
// or temporarily unusable, so a reconnect should precede the next attempt.
// A send error without an SMTP reply code is a network error or timeout (e.g.
// mid-DATA), which leaves the session unusable.
func isConnectionError(err error) bool {
	var se *mail.SendError
	if errors.As(err, &se) {
		return se.Reason == mail.ErrConnCheck || se.IsTemp() || se.ErrorCode() == 0
	}
	return false
}

// isPermanentError reports whether the server rejected the message with a
// permanent (5xx) reply, which a retry cannot fix.
func isPermanentError(err error) bool {
	var se *mail.SendError
	return errors.As(err, &se) && se.ErrorCode() >= 500
}

// deliveryUnknown reports whether err left the outcome unknown: the connection
// failed (no SMTP reply code) after the message data was fully sent.
func deliveryUnknown(err error) bool {
	var se *mail.SendError
	return errors.As(err, &se) && se.Reason == mail.ErrSMTPDataClose && se.ErrorCode() == 0
}

// deliveredDespiteError reports whether the server actually accepted msg even
// though Send returned an error (e.g. a post-delivery RSET failure).
func deliveredDespiteError(msg *mail.Msg, err error) bool {
	if msg.IsDelivered() {
		return true
	}
	var se *mail.SendError
	return errors.As(err, &se) && se.Reason == mail.ErrSMTPReset
}

// createMessage builds a single email message with the given headers and body.
func createMessage(from, toName, toAddr string, cc []string, replyTo, subject, body string) (*mail.Msg, error) {
	m := mail.NewMsg()
	if err := m.From(from); err != nil {
		return nil, fmt.Errorf("invalid From address %q: %w", from, err)
	}
	// Build the To address from parts so net/mail quotes/encodes the display
	// name; formatting it into a string first breaks on names containing '"'.
	to, err := netmail.ParseAddress(toAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid To address %q: %w", toAddr, err)
	}
	if to.Name != "" {
		return nil, fmt.Errorf("invalid To address %q: must be a bare address", toAddr)
	}
	to.Name = toName
	m.AddToMailAddress(to)
	if len(cc) > 0 {
		if err := m.Cc(cc...); err != nil {
			return nil, fmt.Errorf("invalid Cc address(es) %v: %w", cc, err)
		}
	}
	if replyTo != "" {
		if err := m.ReplyTo(replyTo); err != nil {
			return nil, fmt.Errorf("invalid Reply-To address %q: %w", replyTo, err)
		}
	}
	m.Subject(subject)
	m.SetBodyString(mail.TypeTextPlain, body)
	return m, nil
}

// attachFiles adds the given file paths as attachments to msg, verifying
// each is a readable regular file before attaching.
func attachFiles(msg *mail.Msg, attachments []string) error {
	for _, path := range attachments {
		if err := checkAttachment(path); err != nil {
			return fmt.Errorf("attachment %s: %w", path, err)
		}
		msg.AttachFile(path)
	}
	return nil
}
