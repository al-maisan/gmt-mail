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
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/al-maisan/gmt-mail/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mail "github.com/wneessen/go-mail"
)

// renderOne prepares and renders tmpl for one recipient (Jo Doe, jd@example.com).
func renderOne(t *testing.T, tmpl string, data map[string]string) Message {
	t.Helper()
	cfg := &config.MailConfig{Subject: "s", Recipients: []config.Recipient{
		{Email: "jd@example.com", First: "Jo", Last: "Doe", Data: data},
	}}
	msgs, err := PrepMails(cfg, tmpl)
	require.NoError(t, err)
	require.NoError(t, RenderHTML(cfg, tmpl, msgs))
	return msgs[0]
}

// Placeholders are filled in every position goldmark can put them, including
// URLs, where it would percent-encode them only partly (%25ID%25 but %EA%25).
func TestRenderHTML(t *testing.T) {
	data := map[string]string{"ID": "42", "AB": "1", "DE_X": "2"}
	tests := []struct {
		name string
		tmpl string
		want []string
	}{
		{"text", "Hi %FN%", []string{"<p>Hi Jo</p>"}},
		{"heading", "# %FN%", []string{"<h1>Jo</h1>"}},
		{"emphasis", "*%FN%* **%LN%** ~~%EA%~~", []string{"<em>Jo</em> <strong>Doe</strong> <del>jd@example.com</del>"}},
		{"adjacent", "%FN%%LN%", []string{"<p>JoDoe</p>"}},
		{"list", "- %FN%\n- b", []string{"<ul>\n<li>Jo</li>\n<li>b</li>\n</ul>"}},
		{"ordered list", "1. a\n2. b", []string{"<ol>\n<li>a</li>"}},
		{"table", "| %FN% | b |\n|---|---|\n| %LN% | 2 |", []string{"<table>", "<th>Jo</th>", "<td>Doe</td>"}},
		{"link", "[%FN%](https://x.org/u/%ID%?e=%EA%)", []string{`<a href="https://x.org/u/42?e=jd%40example.com">Jo</a>`}},
		{"hex-like keys in url", "[a](https://x.org/%EA%/%AB%/%DE_X%)", []string{`href="https://x.org/jd@example.com/1/2"`}},
		{"image", `![%FN%](https://t.org/%ID%.png "%LN%")`, []string{`<img src="https://t.org/42.png" alt="Jo" title="Doe">`}},
		{"reference link", "[r][1]\n\n[1]: https://x.org/%ID% \"%FN%\"", []string{`<a href="https://x.org/42" title="Jo">r</a>`}},
		{"autolink", "<https://x.org/%ID%>", []string{`<a href="https://x.org/42">https://x.org/42</a>`}},
		{"linkify", "see https://x.org/?id=%ID% now", []string{`<a href="https://x.org/?id=42">https://x.org/?id=42</a>`}},
		{"linkify www", "www.x.org/%ID%", []string{`<a href="http://www.x.org/42">www.x.org/42</a>`}},
		{"code span", "`%FN%`", []string{"<code>Jo</code>"}},
		{"code block", "    %FN%", []string{"<pre><code>Jo\n</code></pre>"}},
		{"encoded form in template stays", "%25FN%25", []string{"<p>%25FN%25</p>"}},
		{"text is escaped", "a < b & c", []string{"a &lt; b &amp; c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := renderOne(t, tt.tmpl, data)
			for _, w := range tt.want {
				assert.Contains(t, m.HTML, w)
			}
		})
	}
}

// A value is always literal text in the HTML part: never Markdown, never HTML,
// never a whole URL. The text part gets the raw value.
func TestRenderHTMLValuesAreLiteral(t *testing.T) {
	tests := []struct {
		name  string
		tmpl  string
		value string
		want  []string
		bad   []string
	}{
		{"link markdown", "Hi %V%", "[x](https://evil)", []string{"<p>Hi [x](https://evil)</p>"}, []string{"<a"}},
		{"image markdown", "Hi %V%", "![](https://t/p.png)", []string{"![](https://t/p.png)"}, []string{"<img"}},
		{"emphasis markdown", "Hi %V%", "*VIP*", []string{"<p>Hi *VIP*</p>"}, []string{"<em>"}},
		{"script", "Hi %V%", "<script>x</script>", []string{"&lt;script&gt;x&lt;/script&gt;"}, []string{"<script"}},
		{"html specials in text", "Hi %V%", `&"<>'`, []string{"Hi &amp;&#34;&lt;&gt;&#39;"}, nil},
		{"html specials in code span", "`%V%`", `&"<>'`, []string{"<code>&amp;&#34;&lt;&gt;&#39;</code>"}, nil},
		{"break out of alt", "![%V%](https://t.org/p.png)", `" onerror="alert(1)`, []string{`alt="&#34; onerror=&#34;alert(1)"`}, []string{`" onerror`}},
		{"break out of title", `[a](https://x.org "%V%")`, `" onclick="alert(1)`, []string{`title="&#34; onclick=&#34;alert(1)"`}, []string{`" onclick`}},
		{"break out of href", "[x](https://x.org/%V%)", `"><img src=x onerror=alert(1)>`, []string{`href="https://x.org/%22%3E%3Cimg%20src=x%20onerror=alert%281%29%3E"`}, []string{"<img", `"><`}},
		{"javascript whole url", "[x](%V%)", "javascript:alert(1)", []string{`href="javascript%3Aalert%281%29"`}, []string{`href="javascript:`}},
		{"url scheme", "[x](%V%)", "https://evil.org/", []string{`href="https%3A%2F%2Fevil.org%2F"`}, nil},
		{"space and slash in url", "[x](https://x.org/%V%)", "a b/c", []string{`href="https://x.org/a%20b%2Fc"`}, nil},
		{"query: no extra parameter", "[x](https://x.org/seat?id=%V%)", "a&admin=1", []string{`href="https://x.org/seat?id=a%26admin%3D1"`}, []string{"&amp;admin"}},
		{"query: space", "[x](https://x.org/seat?id=%V%)", "a b", []string{`href="https://x.org/seat?id=a+b"`}, nil},
		{"query: path part still path-escaped", "[x](https://x.org/%V%?id=%V%)", "a b/c", []string{`href="https://x.org/a%20b%2Fc?id=a+b%2Fc"`}, nil},
		{"query: template parameters kept", "[x](https://x.org/?a=1&b=%V%)", "a&b", []string{`href="https://x.org/?a=1&amp;b=a%26b"`}, nil},
		{"fragment", "[x](https://x.org/p?q=1#s-%V%)", "a b&c=d", []string{`href="https://x.org/p?q=1#s-a%20b%26c%3Dd"`}, nil},
		{"image src query", "![i](https://t.org/p.png?u=%V%)", "a&b=c", []string{`src="https://t.org/p.png?u=a%26b%3Dc"`}, nil},
		{"raw html in template", "Hi <script>x</script> %V%", "v", []string{"Hi <!-- raw HTML omitted -->x<!-- raw HTML omitted --> v"}, []string{"<script"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := renderOne(t, tt.tmpl, map[string]string{"V": tt.value})
			assert.Contains(t, m.Body, tt.value, "the text part gets the raw value")
			for _, w := range tt.want {
				assert.Contains(t, m.HTML, w)
			}
			for _, b := range tt.bad {
				assert.NotContains(t, m.HTML, b)
			}
		})
	}
}

// A placeholder that does not make it into the HTML would silently drop the value.
func TestRenderHTMLPlaceholderNotInHTML(t *testing.T) {
	for _, tmpl := range []string{"Hi <!-- %FN% --> there", "[x](javascript:%FN%)"} {
		t.Run(tmpl, func(t *testing.T) {
			cfg := &config.MailConfig{Subject: "s", Recipients: []config.Recipient{{Email: "a@example.com", First: "Al"}}}
			msgs, err := PrepMails(cfg, tmpl)
			require.NoError(t, err)
			assert.EqualError(t, RenderHTML(cfg, tmpl, msgs), "placeholder %FN% is not in the HTML (inside raw HTML or a dropped link?)")
		})
	}
}

func TestRenderHTMLManyRecipients(t *testing.T) {
	cfg := &config.MailConfig{Subject: "s", Recipients: []config.Recipient{
		{Email: "a@example.com", First: "Al"},
		{Email: "b@example.com", First: "Bo"},
	}}
	msgs, err := PrepMails(cfg, "Hi *%FN%*")
	require.NoError(t, err)
	require.NoError(t, RenderHTML(cfg, "Hi *%FN%*", msgs))
	assert.Equal(t, "<p>Hi <em>Al</em></p>\n", msgs[0].HTML)
	assert.Equal(t, "<p>Hi <em>Bo</em></p>\n", msgs[1].HTML)

	assert.ErrorContains(t, RenderHTML(cfg, "x", msgs[:1]), "1 messages for 2 recipients")
}

// captureSender records each sent message as raw MIME.
type captureSender struct{ raw []string }

func (c *captureSender) Send(m *mail.Msg) error {
	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return err
	}
	c.raw = append(c.raw, buf.String())
	return nil
}
func (c *captureSender) Reconnect() error { return nil }
func (c *captureSender) Close() error     { return nil }

// mimePart is a decoded leaf part, or a multipart with its children.
type mimePart struct {
	typ   string
	body  string
	parts []mimePart
}

// parseMIME decodes the part with the given header and body.
func parseMIME(t *testing.T, h map[string][]string, r io.Reader) mimePart {
	t.Helper()
	get := func(k string) string {
		if v := h[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	typ, params, err := mime.ParseMediaType(get("Content-Type"))
	require.NoError(t, err)
	p := mimePart{typ: typ}
	if strings.HasPrefix(typ, "multipart/") {
		mr := multipart.NewReader(r, params["boundary"])
		for {
			sub, err := mr.NextRawPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			p.parts = append(p.parts, parseMIME(t, sub.Header, sub))
		}
		return p
	}
	switch strings.ToLower(get("Content-Transfer-Encoding")) {
	case "quoted-printable":
		r = quotedprintable.NewReader(r)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, r)
	}
	bs, err := io.ReadAll(r)
	require.NoError(t, err)
	p.body = strings.ReplaceAll(string(bs), "\r\n", "\n") // MIME lines end in CRLF
	return p
}

func sendAndParse(t *testing.T, m Message) mimePart {
	t.Helper()
	sender := &captureSender{}
	var buf bytes.Buffer
	res := NewBatchSender(&buf, sender, config.MailConfig{From: "s@example.com"}, SendOptions{}).SendAll([]Message{m})
	require.Equal(t, 1, res.Sent, buf.String())
	require.Len(t, sender.raw, 1)
	nm, err := netmail.ReadMessage(strings.NewReader(sender.raw[0]))
	require.NoError(t, err)
	return parseMIME(t, nm.Header, nm.Body)
}

func TestSendMarkdownMIME(t *testing.T) {
	att := filepath.Join(t.TempDir(), "a.txt")
	require.NoError(t, os.WriteFile(att, []byte("data"), 0o600))
	body := "# Hi Jo\n\n- [site](https://example.com)\n"
	msgs := []Message{renderOne(t, "# Hi %FN%\n\n- [site](https://example.com)\n", nil)}
	require.NotEmpty(t, msgs[0].HTML)
	alt := func(t *testing.T, p mimePart) {
		require.Equal(t, "multipart/alternative", p.typ)
		require.Len(t, p.parts, 2)
		assert.Equal(t, "text/plain", p.parts[0].typ)
		assert.Equal(t, body, p.parts[0].body)
		assert.Equal(t, "text/html", p.parts[1].typ)
		assert.Equal(t, msgs[0].HTML, p.parts[1].body)
	}

	t.Run("text and html", func(t *testing.T) { alt(t, sendAndParse(t, msgs[0])) })
	t.Run("with attachment", func(t *testing.T) {
		m := msgs[0]
		m.Attachments = []string{att}
		p := sendAndParse(t, m)
		require.Equal(t, "multipart/mixed", p.typ)
		require.Len(t, p.parts, 2)
		alt(t, p.parts[0])
		assert.Equal(t, "data", p.parts[1].body)
	})
	t.Run("plain text without markdown", func(t *testing.T) {
		p := sendAndParse(t, Message{Name: "Jo", Address: "jd@example.com", Subject: "s", Body: body})
		assert.Equal(t, "text/plain", p.typ)
		assert.Equal(t, body, p.body)
	})
}
