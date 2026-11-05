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
	"bytes"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"strings"
	"testing"

	"github.com/al-maisan/gmt-mail/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mail "github.com/wneessen/go-mail"
	"github.com/wneessen/go-mail/smtp"
)

func TestCreateMessage(t *testing.T) {
	msg, err := createMessage(
		"sender@example.com",
		"John Doe", "jd@example.com",
		nil, "", "Hello!", "Body text",
	)
	require.NoError(t, err)
	assert.NotNil(t, msg)
}

func TestCreateMessageWithCcAndReplyTo(t *testing.T) {
	msg, err := createMessage(
		"sender@example.com",
		"John Doe", "jd@example.com",
		[]string{"cc1@example.com", "cc2@example.com"},
		"reply@example.com",
		"Test", "Body",
	)
	require.NoError(t, err)
	assert.NotNil(t, msg)
}

func TestCreateMessageUTF8Name(t *testing.T) {
	msg, err := createMessage(
		"sender@example.com",
		"abc ähm", "abc@example.com",
		nil, "", "Hello!", "Body",
	)
	require.NoError(t, err)
	assert.NotNil(t, msg)
}

func TestCreateMessageInvalidFrom(t *testing.T) {
	_, err := createMessage(
		"not-an-email",
		"A", "a@b.com",
		nil, "", "s", "b",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid From")
}

func TestCreateMessageInvalidTo(t *testing.T) {
	_, err := createMessage(
		"sender@example.com",
		"A", "not-an-email",
		nil, "", "s", "b",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid To")
}

func TestAttachFilesNonexistent(t *testing.T) {
	msg, err := createMessage("s@s.com", "A", "a@b.com", nil, "", "s", "b")
	require.NoError(t, err)
	err = attachFiles(msg, []string{"/nonexistent/file.txt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "attachment /nonexistent/file.txt")
}

func TestAttachFilesEmpty(t *testing.T) {
	msg, err := createMessage("s@s.com", "A", "a@b.com", nil, "", "s", "b")
	require.NoError(t, err)
	assert.NoError(t, attachFiles(msg, nil))
}

func TestLoadSMTPCredentials(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "all missing",
			env:     map[string]string{"SMTP_HOST": "", "SMTP_PORT": "", "SENDER_EMAIL": "", "SENDER_PASSWORD": ""},
			wantErr: "SMTP_HOST",
		},
		{
			name:    "partial missing",
			env:     map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "587", "SENDER_EMAIL": "", "SENDER_PASSWORD": "secret"},
			wantErr: "SENDER_EMAIL",
		},
		{
			name:    "bad port",
			env:     map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "abc", "SENDER_EMAIL": "u@e.com", "SENDER_PASSWORD": "secret"},
			wantErr: "SMTP_PORT",
		},
		{
			name:    "port zero",
			env:     map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "0", "SENDER_EMAIL": "u@e.com", "SENDER_PASSWORD": "secret"},
			wantErr: "SMTP_PORT",
		},
		{
			name:    "negative port",
			env:     map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "-587", "SENDER_EMAIL": "u@e.com", "SENDER_PASSWORD": "secret"},
			wantErr: "SMTP_PORT",
		},
		{
			name:    "port above 65535",
			env:     map[string]string{"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "65536", "SENDER_EMAIL": "u@e.com", "SENDER_PASSWORD": "secret"},
			wantErr: "SMTP_PORT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := LoadSMTPCredentials()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCreateMessageWithReplyTo(t *testing.T) {
	msg, err := createMessage(
		"sender@example.com",
		"John Doe", "jd@example.com",
		nil,
		`"Support" <support@example.com>`,
		"Test", "Body",
	)
	require.NoError(t, err)
	assert.NotNil(t, msg)
}

func TestCreateMessageInvalidCc(t *testing.T) {
	_, err := createMessage(
		"sender@example.com",
		"A", "a@b.com",
		[]string{"not-an-email"}, "",
		"s", "b",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Cc")
}

func TestCreateMessageInvalidReplyTo(t *testing.T) {
	_, err := createMessage(
		"sender@example.com",
		"A", "a@b.com",
		nil, "not-an-email",
		"s", "b",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Reply-To")
}

func TestAttachFilesValid(t *testing.T) {
	msg, err := createMessage("s@s.com", "A", "a@b.com", nil, "", "s", "b")
	require.NoError(t, err)

	tmpFile := t.TempDir() + "/test.txt"
	require.NoError(t, os.WriteFile(tmpFile, []byte("content"), 0o644))
	assert.NoError(t, attachFiles(msg, []string{tmpFile}))
}

// mockSender records Send calls and returns a configurable error.
type mockSender struct {
	sent    int
	sendErr error
	closed  bool
}

func (m *mockSender) Send(_ *mail.Msg) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent++
	return nil
}

func (m *mockSender) Reconnect() error { return nil }

func (m *mockSender) Close() error {
	m.closed = true
	return nil
}

func TestNewSMTPSenderConnectionError(t *testing.T) {
	creds := SMTPCredentials{
		Host:     "localhost",
		Port:     19999,
		User:     "user",
		Password: "pass",
	}
	_, err := NewSMTPSender(creds, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestSendAllProgressAlignment(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := make([]Message, 12)
	for i := range msgs {
		msgs[i] = Message{Name: fmt.Sprintf("R%d", i+1), Address: fmt.Sprintf("r%d@example.com", i+1), Subject: "Hi", Body: "Hello"}
	}

	var buf bytes.Buffer
	NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	lines := strings.Split(buf.String(), "\n")

	// First and last progress prefixes should be the same width
	assert.Contains(t, lines[0], "[ 1/12]")
	assert.Contains(t, lines[11], "[12/12]")

	// All prefix widths should match
	for i := 0; i < 12; i++ {
		assert.Contains(t, lines[i], "/12]")
	}
}

func TestSendAllSuccess(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
		{Name: "Jane", Address: "jane@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 2, result.Sent)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 2, sender.sent)
	assert.Contains(t, buf.String(), "- John <jd@example.com>")
	assert.Contains(t, buf.String(), "- Jane <jane@example.com>")
}

func TestSendAllSendError(t *testing.T) {
	sender := &mockSender{sendErr: fmt.Errorf("connection reset")}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 0, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Contains(t, buf.String(), "! John <jd@example.com>")
	assert.Contains(t, buf.String(), "connection reset")
}

func TestSendAllInvalidFrom(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "not-an-email"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 0, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Contains(t, buf.String(), "failed to create")
	assert.Equal(t, 0, sender.sent)
}

func TestSendAllMissingAttachment(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello", Attachments: []string{"/nonexistent/file.txt"}},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 0, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Contains(t, buf.String(), "failed to attach")
}

func TestSendAllMixedResults(t *testing.T) {
	sender := &failNthSender{failOn: 2}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
		{Name: "Jane", Address: "jane@example.com", Subject: "Hi", Body: "Hello"},
		{Name: "Bob", Address: "bob@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 2, result.Sent)
	assert.Equal(t, 1, result.Failed)
}

// failNthSender fails on the Nth Send call, succeeds on all others.
type failNthSender struct {
	calls  int
	failOn int
}

func (f *failNthSender) Send(_ *mail.Msg) error {
	f.calls++
	if f.calls == f.failOn {
		return fmt.Errorf("simulated failure on send #%d", f.failOn)
	}
	return nil
}

func (f *failNthSender) Reconnect() error { return nil }

func (f *failNthSender) Close() error { return nil }

func TestSendAllRetrySuccess(t *testing.T) {
	// Fails on first call, succeeds on second (the retry)
	sender := &failNthSender{failOn: 1}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 1}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 0, result.Failed)
	assert.Contains(t, buf.String(), "retrying")
	assert.Contains(t, buf.String(), "- John <jd@example.com>")
}

func TestSendAllRetryExhausted(t *testing.T) {
	// Always fails — retries should be exhausted
	sender := &mockSender{sendErr: fmt.Errorf("permanent error")}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 2}).SendAll(msgs)
	assert.Equal(t, 0, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 2, strings.Count(buf.String(), "retrying"))
	assert.Contains(t, buf.String(), "permanent error")
}

func TestCheckAttachments(t *testing.T) {
	existing := t.TempDir() + "/present.txt"
	require.NoError(t, os.WriteFile(existing, []byte("x"), 0o644))

	// Existing per-recipient attachment: no error.
	assert.NoError(t, CheckAttachments([]Message{
		{Address: "a@b.com", Attachments: []string{existing}},
	}))

	// Missing per-recipient attachment: reported up front with recipient context.
	err := CheckAttachments([]Message{
		{Address: "a@b.com", Attachments: []string{existing}},
		{Address: "c@d.com", Attachments: []string{"/nonexistent/missing.pdf"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/nonexistent/missing.pdf")
	assert.Contains(t, err.Error(), "c@d.com")
}

// connDropSender fails the first send with a dropped-connection error, then
// succeeds; it records how many times Reconnect was invoked.
type connDropSender struct {
	sends   int
	reconns int
}

func (c *connDropSender) Send(_ *mail.Msg) error {
	c.sends++
	if c.sends == 1 {
		return &mail.SendError{Reason: mail.ErrConnCheck}
	}
	return nil
}
func (c *connDropSender) Reconnect() error { c.reconns++; return nil }
func (c *connDropSender) Close() error     { return nil }

func TestSendAllReconnectsOnDroppedConnection(t *testing.T) {
	sender := &connDropSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"}}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 1}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1, sender.reconns, "should re-dial after a dropped connection")
	assert.Equal(t, 2, sender.sends)
}

// resetErrSender always returns a post-delivery RSET error — the server has
// already accepted the message.
type resetErrSender struct{ sends int }

func (r *resetErrSender) Send(_ *mail.Msg) error {
	r.sends++
	return &mail.SendError{Reason: mail.ErrSMTPReset}
}
func (r *resetErrSender) Reconnect() error { return nil }
func (r *resetErrSender) Close() error     { return nil }

func TestSendAllDeliveredDespiteResetError(t *testing.T) {
	sender := &resetErrSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"}}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 2}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent, "delivered message must count as sent, not failed")
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 1, sender.sends, "must not retry an already-delivered message")
}

func TestSendAllNegativeRetriesStillSends(t *testing.T) {
	// A negative Retries must not skip the send and falsely report success.
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: -1}).SendAll(msgs)
	assert.Equal(t, 1, sender.sent)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 0, result.Failed)
}

func TestSendAllWithReplyTo(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{
		From:    "sender@example.com",
		ReplyTo: "reply@example.com",
	}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 0, result.Failed)
}

func TestSendAllWithCc(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello", Cc: []string{"cc@example.com"}},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 0, result.Failed)
}

func TestSendAllEmpty(t *testing.T) {
	sender := &mockSender{}
	cfg := config.MailConfig{From: "sender@example.com"}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{}).SendAll(nil)
	assert.Equal(t, 0, result.Sent)
	assert.Equal(t, 0, result.Failed)
	assert.Empty(t, buf.String())
}

func TestLoadSMTPCredentialsValid(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SENDER_EMAIL", "user@example.com")
	t.Setenv("SENDER_PASSWORD", "secret")

	creds, err := LoadSMTPCredentials()
	assert.NoError(t, err)
	assert.Equal(t, "smtp.example.com", creds.Host)
	assert.Equal(t, 587, creds.Port)
	assert.Equal(t, "user@example.com", creds.User)
	assert.Equal(t, "secret", creds.Password)
}

func TestCreateMessageNameWithQuotes(t *testing.T) {
	msg, err := createMessage("s@s.com", `A" <evil@x.com>, "B`, "a@b.com", nil, "", "s", "b")
	require.NoError(t, err)
	rcpts, err := msg.GetRecipients()
	require.NoError(t, err)
	assert.Equal(t, []string{"<a@b.com>"}, rcpts, "a quote in the name must not add recipients")
}

func TestCreateMessageToWithDisplayName(t *testing.T) {
	_, err := createMessage("s@s.com", "A", "X <a@b.com>", nil, "", "s", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a bare address")
}

func TestCheckAttachmentRejectsNonRegular(t *testing.T) {
	dir := t.TempDir()
	err := CheckAttachments([]Message{{Address: "a@b.com", Attachments: []string{dir}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")

	msg, err := createMessage("s@s.com", "A", "a@b.com", nil, "", "s", "b")
	require.NoError(t, err)
	assert.Error(t, attachFiles(msg, []string{dir}))
}

func TestCheckAttachmentRejectsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read any file")
	}
	f := t.TempDir() + "/secret.txt"
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o000))
	assert.Error(t, CheckAttachments([]Message{{Address: "a@b.com", Attachments: []string{f}}}))
}

// lostDataReplySender fails the first send like a connection lost after the
// message data was sent (final reply never arrived), then succeeds.
type lostDataReplySender struct{ connDropSender }

func (l *lostDataReplySender) Send(_ *mail.Msg) error {
	l.sends++
	if l.sends == 1 {
		return &mail.SendError{Reason: mail.ErrSMTPDataClose}
	}
	return nil
}

func TestSendAllNoResendWhenDeliveryUnknown(t *testing.T) {
	sender := &lostDataReplySender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{
		{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"},
		{Name: "Jane", Address: "ja@example.com", Subject: "Hi", Body: "Hello"},
	}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 1}).SendAll(msgs)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 2, sender.sends, "a message that may have been delivered must not be re-sent")
	assert.Equal(t, 1, sender.reconns, "must re-dial for the next recipient")
	assert.Contains(t, buf.String(), "may have been delivered")
	assert.NotContains(t, buf.String(), "retrying")
}

// dropMidDataSender fails the first send like a connection lost mid-DATA (no
// SMTP reply code), then succeeds.
type dropMidDataSender struct{ connDropSender }

func (d *dropMidDataSender) Send(_ *mail.Msg) error {
	d.sends++
	if d.sends == 1 {
		return &mail.SendError{Reason: mail.ErrWriteContent}
	}
	return nil
}

func TestSendAllReconnectsAfterMidDataDrop(t *testing.T) {
	sender := &dropMidDataSender{}
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"}}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 1}).SendAll(msgs)
	assert.Equal(t, 1, result.Sent)
	assert.Equal(t, 1, sender.reconns, "a mid-DATA network error must trigger a reconnect")
}

// fakeSMTP runs a scripted SMTP server on conn: it greets, then answers each
// command with reply(cmd); an empty reply drops the connection.
func fakeSMTP(conn net.Conn, reply func(cmd string) string) {
	defer conn.Close() //nolint:errcheck
	tp := textproto.NewConn(conn)
	if tp.PrintfLine("220 fake ESMTP") != nil {
		return
	}
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		r := reply(strings.ToUpper(line))
		if r == "" || tp.PrintfLine("%s", r) != nil {
			return
		}
	}
}

// newFakeSMTPSender returns an smtpSender connected to a fakeSMTP server.
func newFakeSMTPSender(t *testing.T, reply func(cmd string) string) *smtpSender {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	go fakeSMTP(serverConn, reply)
	conn, err := smtp.NewClient(clientConn, "localhost")
	require.NoError(t, err)
	client, err := mail.NewClient("localhost")
	require.NoError(t, err)
	return &smtpSender{client: client, conn: conn}
}

func TestSendAllNoRetryOnPermanentError(t *testing.T) {
	rcpts := 0
	sender := newFakeSMTPSender(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "RCPT") {
			rcpts++
			return "550 5.1.1 no such user"
		}
		return "250 ok"
	})
	cfg := config.MailConfig{From: "sender@example.com"}
	msgs := []Message{{Name: "John", Address: "jd@example.com", Subject: "Hi", Body: "Hello"}}

	var buf bytes.Buffer
	result := NewBatchSender(&buf, sender, cfg, SendOptions{Retries: 2}).SendAll(msgs)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 1, rcpts, "a permanent 5xx rejection must not be retried")
	assert.NotContains(t, buf.String(), "retrying")
}

func TestSMTPSenderCloseDropsSocketWhenQuitFails(t *testing.T) {
	sender := newFakeSMTPSender(t, func(cmd string) string {
		if strings.HasPrefix(cmd, "QUIT") {
			return "" // connection dies before replying
		}
		return "250 ok"
	})
	assert.Error(t, sender.Close())
	assert.False(t, sender.conn.HasConnection(), "socket must be closed even though QUIT failed")
}
