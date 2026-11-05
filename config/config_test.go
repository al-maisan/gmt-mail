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

package config

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sortRecipients(r []Recipient) {
	sort.Slice(r, func(i, j int) bool {
		return r[i].Email > r[j].Email
	})
}

func parseTestConfig(t *testing.T, input []byte) MailConfig {
	t.Helper()
	cfg, err := Parse(input)
	require.NoError(t, err)
	return cfg
}

func TestParseDefault(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "Frodo Baggins <rts@example.com>"
subject = "Hello %FN%!"

[[recipients]]
email = "jd@example.com"
first = "John"
last = "Doe Jr."
data = { ORG = "EFF", TITLE = "PhD" }

[[recipients]]
email = "mm@gmail.com"
first = "Mickey"
last = "Mouse"
data = { ORG = "Disney" }
`))

	assert.Equal(t, "Frodo Baggins <rts@example.com>", cfg.From)
	assert.Empty(t, cfg.Cc)

	expected := []Recipient{
		{Email: "jd@example.com", First: "John", Last: "Doe Jr.", Data: map[string]string{"ORG": "EFF", "TITLE": "PhD"}},
		{Email: "mm@gmail.com", First: "Mickey", Last: "Mouse", Data: map[string]string{"ORG": "Disney"}},
	}
	sortRecipients(expected)
	sortRecipients(cfg.Recipients)
	assert.Equal(t, expected, cfg.Recipients)
}

func TestParseMissingFrom(t *testing.T) {
	_, err := Parse([]byte(`
[general]
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required key 'from'")
}

func TestParseMissingSubject(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "x <x@x.com>"
[[recipients]]
email = "a@b.com"
first = "A"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required key 'subject'")
}

func TestParseNoRecipients(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "Frodo Baggins <rts@example.com>"
subject = "Hello %FN%!"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no [[recipients]] entries found")
}

func TestParseInvalidTOML(t *testing.T) {
	_, err := Parse([]byte(`this is not valid TOML {{{`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TOML syntax error")
}

func TestParseFull(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "Frodo Baggins <rts@example.com>"
cc = ["weirdo@nsb.gov", "cc@example.com"]
reply_to = "John Doe <jd@mail.com>"
subject = "Hello %FN%!"

[[recipients]]
email = "jd@example.com"
first = "John"
last = "Doe Jr."
data = { ORG = "EFF", TITLE = "PhD" }

[[recipients]]
email = "mm@gmail.com"
first = "Mickey"
last = "Mouse"
data = { ORG = "Disney" }
`))

	assert.Equal(t, "Frodo Baggins <rts@example.com>", cfg.From)
	assert.Equal(t, "John Doe <jd@mail.com>", cfg.ReplyTo)
	assert.Equal(t, "Hello %FN%!", cfg.Subject)
	assert.Equal(t, []string{"weirdo@nsb.gov", "cc@example.com"}, cfg.Cc)

	expected := []Recipient{
		{Email: "jd@example.com", First: "John", Last: "Doe Jr.", Data: map[string]string{"ORG": "EFF", "TITLE": "PhD"}},
		{Email: "mm@gmail.com", First: "Mickey", Last: "Mouse", Data: map[string]string{"ORG": "Disney"}},
	}
	sortRecipients(expected)
	sortRecipients(cfg.Recipients)
	assert.Equal(t, expected, cfg.Recipients)
}

func TestParseRecipientSingleName(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "madonna@example.com"
first = "Madonna"
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, "Madonna", cfg.Recipients[0].First)
	assert.Equal(t, "", cfg.Recipients[0].Last)
	assert.Equal(t, "madonna@example.com", cfg.Recipients[0].Email)
}

func TestParseRecipientMissingEmail(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
first = "John"
last = "Doe"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Email")
}

func TestParseRecipientMissingFirst(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "jd@example.com"
last = "Doe"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "First")
}

func TestParseRecipientInvalidEmail(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "not-an-email"
first = "John"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email")
}

func TestParseRecipientCcReplace(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
cc = ["override@cc.com"]
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, []string{"override@cc.com"}, cfg.Recipients[0].Cc)
}

func TestParseRecipientCcAppend(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
cc_extra = ["extra@cc.com"]
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, []string{"extra@cc.com"}, cfg.Recipients[0].CcExtra)
}

func TestParseRecipientAttachReplace(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
attachments = ["local.txt"]
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, []string{"local.txt"}, cfg.Recipients[0].Attachments)
}

func TestParseRecipientAttachAppend(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
attachments_extra = ["extra.pdf"]
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, []string{"extra.pdf"}, cfg.Recipients[0].AttachmentsExtra)
}

func TestParseDataKeysUppercased(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
data = { org = "EFF" }
`))
	require.Len(t, cfg.Recipients, 1)
	assert.Equal(t, "EFF", cfg.Recipients[0].Data["ORG"])
}

func TestParseDataKeyCaseCollision(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
data = { url = "a", URL = "b" }
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collide after upper-casing")
}

func TestParseDataKeyReservedCollision(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "test <t@example.com>"
subject = "test"
[[recipients]]
email = "a@b.com"
first = "A"
data = { fn = "Company" }
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved placeholder %FN%")
}

func TestParseWithAttachments(t *testing.T) {
	tmpFile := t.TempDir() + "/attach.txt"
	require.NoError(t, os.WriteFile(tmpFile, []byte("x"), 0o644))

	cfg := parseTestConfig(t, []byte(`
[general]
from = "a <a@a.com>"
subject = "test"
attachments = ["`+tmpFile+`"]
[[recipients]]
email = "a@b.com"
first = "Alice"
`))
	assert.Equal(t, []string{tmpFile}, cfg.Attachments)
}

func TestParseWhitespaceSubject(t *testing.T) {
	_, err := Parse([]byte(`
[general]
from = "a <a@a.com>"
subject = "   "
[[recipients]]
email = "a@b.com"
first = "Alice"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subject must not be empty")
}

func TestSampleConfigParses(t *testing.T) {
	cfg, err := Parse([]byte(SampleConfig("0.0.0")))
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.From)
	assert.NotEmpty(t, cfg.Subject)
	assert.NotEmpty(t, cfg.Recipients)
	for _, r := range cfg.Recipients {
		assert.NotEmpty(t, r.Email)
		assert.NotEmpty(t, r.First)
	}
}

func TestSampleTemplateNotEmpty(t *testing.T) {
	tmpl := SampleTemplate()
	assert.NotEmpty(t, tmpl)
	assert.Contains(t, tmpl, "%FN%")
	assert.Contains(t, tmpl, "%LN%")
}

func TestParseRejects(t *testing.T) {
	const general = "[general]\nfrom = \"a <a@a.com>\"\nsubject = \"s\"\n"
	tests := []struct {
		name, input, want string
	}{
		{"unknown general key", general + "atachments = [\"x.pdf\"]\n[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n", "unknown key(s): general.atachments"},
		{"unknown recipient key", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\nattachment = [\"x.pdf\"]\n", "unknown key(s): recipients.attachment"},
		{"blank first", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \" \"\n", "recipient \"a@b.com\": 'first' must not be blank"},
		{"cc and cc_extra", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ncc = [\"c@d.com\"]\ncc_extra = [\"e@f.com\"]\n", "either 'cc' or 'cc_extra'"},
		{"attachments and attachments_extra", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\nattachments = [\"x\"]\nattachments_extra = [\"y\"]\n", "either 'attachments' or 'attachments_extra'"},
		{"invalid from", "[general]\nfrom = \"not an address\"\nsubject = \"s\"\n[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n", "invalid address in 'from'"},
		{"invalid reply_to", general + "reply_to = \"bad\"\n[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n", "invalid address in 'reply_to'"},
		{"invalid general cc", general + "cc = [\"bad cc\"]\n[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n", "invalid address in 'cc'"},
		{"invalid recipient cc", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ncc = [\"bad\"]\n", "invalid address in 'cc'"},
		{"invalid recipient cc_extra", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ncc_extra = [\"bad\"]\n", "invalid address in 'cc_extra'"},
		{"empty data key", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ndata = { \"\" = \"x\" }\n", "invalid data key \"\""},
		{"data key with percent", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ndata = { \"A%B\" = \"x\" }\n", "invalid data key \"A%B\""},
		{"data key with dash", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\ndata = { \"my-key\" = \"x\" }\n", "invalid data key \"my-key\""},
		{"duplicate recipient", general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n[[recipients]]\nemail = \"A@B.com\"\nfirst = \"B\"\n", "duplicate recipient \"A@B.com\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseAcceptsValidAddressesAndKeys(t *testing.T) {
	cfg := parseTestConfig(t, []byte(`
[general]
from = '"Frodo Baggins" <rts@example.com>'
reply_to = "reply@example.com"
cc = ["Gandalf <g@example.com>"]
subject = "s"
[[recipients]]
email = "a@b.com"
first = "A"
cc_extra = ["x@y.com"]
data = { org_2 = "EFF" }
`))
	assert.Equal(t, "EFF", cfg.Recipients[0].Data["ORG_2"])
}
