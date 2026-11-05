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
	"testing"

	"github.com/al-maisan/gmt-mail/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name string
		r    config.Recipient
		text string
		want string
	}{
		{
			name: "all placeholders",
			r:    config.Recipient{Email: "jd@example.com", First: "John", Last: "Doe", Data: map[string]string{"ORG": "EFF"}},
			text: "Hello %FN% %LN%, you work at %ORG%. Email: %EA%",
			want: "Hello John Doe, you work at EFF. Email: jd@example.com",
		},
		{
			name: "no custom data",
			r:    config.Recipient{Email: "a@b.com", First: "Alice", Last: "Bob", Data: map[string]string{}},
			text: "%FN% %LN%",
			want: "Alice Bob",
		},
		{
			name: "unknown placeholder left as-is",
			r:    config.Recipient{Email: "a@b.com", First: "Alice", Last: "Bob", Data: map[string]string{}},
			text: "Hello %FN%, your title is %TITLE%",
			want: "Hello Alice, your title is %TITLE%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, substituteVariables(tt.r, tt.text))
		})
	}
}

func TestPrepMailsBasic(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "Hi %FN%!",
		Recipients: []config.Recipient{
			{Email: "jd@example.com", First: "John", Last: "Doe", Data: map[string]string{}},
		},
	}
	mails, err := PrepMails(&cfg, "Hello %FN% %LN%")
	require.NoError(t, err)
	assert.Len(t, mails, 1)
	assert.Equal(t, "John Doe", mails[0].Name)
	assert.Equal(t, "jd@example.com", mails[0].Address)
	assert.Equal(t, "Hi John!", mails[0].Subject)
	assert.Equal(t, "Hello John Doe", mails[0].Body)
}

func TestPrepMailsSingleName(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "Hi %FN%!",
		Recipients: []config.Recipient{
			{Email: "m@example.com", First: "Madonna", Last: "", Data: map[string]string{}},
		},
	}
	mails, err := PrepMails(&cfg, "Hello %FN%")
	require.NoError(t, err)
	assert.Len(t, mails, 1)
	assert.Equal(t, "Madonna", mails[0].Name)
}

func TestPrepMailsPerRecipientOverrides(t *testing.T) {
	tests := []struct {
		name            string
		globalCc        []string
		globalAttach    []string
		recipient       config.Recipient
		wantCc          []string
		wantAttachments []string
	}{
		{
			name:      "cc replace",
			globalCc:  []string{"global@cc.com"},
			recipient: config.Recipient{Email: "a@b.com", First: "A", Last: "B", Cc: []string{"override@cc.com"}},
			wantCc:    []string{"override@cc.com"},
		},
		{
			name:      "cc append",
			globalCc:  []string{"global@cc.com"},
			recipient: config.Recipient{Email: "a@b.com", First: "A", Last: "B", CcExtra: []string{"extra@cc.com"}},
			wantCc:    []string{"global@cc.com", "extra@cc.com"},
		},
		{
			name:      "cc global copied",
			globalCc:  []string{"cc1@example.com", "cc2@example.com"},
			recipient: config.Recipient{Email: "a@b.com", First: "A", Last: "B"},
			wantCc:    []string{"cc1@example.com", "cc2@example.com"},
		},
		{
			name:      "cc deduped case-insensitively, first spelling kept",
			globalCc:  []string{"a@x.org"},
			recipient: config.Recipient{Email: "s@x.org", First: "S", CcExtra: []string{"a@x.org", "A@X.ORG", "\"Ann\" <a@x.org>", "b@x.org"}},
			wantCc:    []string{"a@x.org", "b@x.org"},
		},
		{
			name:      "recipient not cc'd on own mail",
			globalCc:  []string{"S@X.org", "a@x.org"},
			recipient: config.Recipient{Email: "s@x.org", First: "S", CcExtra: []string{"Sam <s@x.org>"}},
			wantCc:    []string{"a@x.org"},
		},
		{
			name:      "cc only the recipient: none left",
			recipient: config.Recipient{Email: "s@x.org", First: "S", Cc: []string{"s@x.org"}},
		},
		{
			name:            "attachment replace",
			globalAttach:    []string{"global.txt"},
			recipient:       config.Recipient{Email: "a@b.com", First: "A", Last: "B", Attachments: []string{"local.txt"}},
			wantAttachments: []string{"local.txt"},
		},
		{
			name:            "attachment append",
			globalAttach:    []string{"global.txt"},
			recipient:       config.Recipient{Email: "a@b.com", First: "A", Last: "B", AttachmentsExtra: []string{"extra.txt"}},
			wantAttachments: []string{"global.txt", "extra.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.MailConfig{
				Subject:     "Test",
				Cc:          tt.globalCc,
				Attachments: tt.globalAttach,
				Recipients:  []config.Recipient{tt.recipient},
			}
			mails, err := PrepMails(&cfg, "body")
			require.NoError(t, err)
			require.Len(t, mails, 1)
			assert.Equal(t, tt.wantCc, mails[0].Cc)
			assert.Equal(t, tt.wantAttachments, mails[0].Attachments)
		})
	}
}

func TestPrepMailsDoesNotMutateConfig(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "Test",
		Cc:      []string{"global@cc.com"},
		Recipients: []config.Recipient{
			{Email: "a@b.com", First: "A", Last: "B", Cc: []string{"override@cc.com"}, Data: map[string]string{"ORG": "EFF"}},
		},
	}
	_, err := PrepMails(&cfg, "body")
	require.NoError(t, err)
	assert.Equal(t, []string{"override@cc.com"}, cfg.Recipients[0].Cc)
	assert.Equal(t, "EFF", cfg.Recipients[0].Data["ORG"])
}

func TestPrepMailsUnresolvedPlaceholders(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "Hi %FN% from %DEPT%!",
		Recipients: []config.Recipient{
			{Email: "a@b.com", First: "Alice", Last: "Bob", Data: map[string]string{}},
		},
	}
	_, err := PrepMails(&cfg, "Hello %FN%, your role is %ROLE%")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "%DEPT%")
	assert.Contains(t, err.Error(), "%ROLE%")
}

func TestPrepMailsBlankAfterSubstitution(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "%LN%",
		Recipients: []config.Recipient{
			{Email: "a@b.com", First: "Alice", Data: map[string]string{"NOTE": " "}},
			{Email: "c@d.com", First: "Carl", Last: "Doe", Data: map[string]string{"NOTE": "hi"}},
		},
	}
	_, err := PrepMails(&cfg, "%NOTE%")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient 'a@b.com': subject is blank after substitution")
	assert.Contains(t, err.Error(), "recipient 'a@b.com': body is blank after substitution")
	assert.NotContains(t, err.Error(), "c@d.com")
}

func TestPrepMailsNoErrorWhenAllResolved(t *testing.T) {
	cfg := config.MailConfig{
		Subject: "Hi %FN%!",
		Recipients: []config.Recipient{
			{Email: "a@b.com", First: "Alice", Last: "Bob", Data: map[string]string{"ORG": "EFF"}},
		},
	}
	_, err := PrepMails(&cfg, "Hello %FN% from %ORG%")
	require.NoError(t, err)
}

func TestPrepMailsPlaceholderLikeValueNotFlagged(t *testing.T) {
	// A substituted value that merely looks like a placeholder must not be
	// misreported as unresolved (regression: batch abort on "50%OFF%deal").
	cfg := config.MailConfig{
		Subject: "Deal for %FN%",
		Recipients: []config.Recipient{
			{Email: "a@b.com", First: "Alice", Last: "Bob", Data: map[string]string{"PROMO": "50%OFF%deal"}},
		},
	}
	mails, err := PrepMails(&cfg, "Hello %FN%, code: %PROMO%")
	require.NoError(t, err)
	require.Len(t, mails, 1)
	assert.Equal(t, "Hello Alice, code: 50%OFF%deal", mails[0].Body)
}

func TestResolveOverride(t *testing.T) {
	tests := []struct {
		name    string
		global  []string
		replace []string
		extra   []string
		want    []string
	}{
		{"no override", []string{"a", "b"}, nil, nil, []string{"a", "b"}},
		{"replace", []string{"a", "b"}, []string{"x", "y"}, nil, []string{"x", "y"}},
		{"append", []string{"a"}, nil, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"empty global replace", nil, []string{"x"}, nil, []string{"x"}},
		{"empty global append", nil, nil, []string{"x"}, []string{"x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveOverride(tt.global, tt.replace, tt.extra))
		})
	}
}

func TestSampleConfigAndTemplateIntegration(t *testing.T) {
	cfg, err := config.Parse([]byte(config.SampleConfig("0.0.0")))
	require.NoError(t, err)

	mails, err := PrepMails(&cfg, config.SampleTemplate())
	require.NoError(t, err, "sample config+template should produce no errors")
	require.NotEmpty(t, mails, "should produce at least one mail")
	for _, m := range mails {
		assert.NotEmpty(t, m.Name)
		assert.NotEmpty(t, m.Address)
		assert.NotEmpty(t, m.Subject)
		assert.NotEmpty(t, m.Body)
	}
}
