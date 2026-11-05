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
	"crypto/rand"
	"fmt"
	"html"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/al-maisan/gmt-mail/config"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// md renders CommonMark plus GFM tables, strikethrough and autolinks. It keeps
// goldmark's safe default (no html.WithUnsafe): raw HTML is omitted and
// dangerous link URLs (e.g. javascript:) are dropped.
var md = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.Linkify))

// reURLAttr matches the URL attributes goldmark emits. A '"' inside a value is
// always written as &quot;, so [^"]* ends at the closing quote.
var reURLAttr = regexp.MustCompile(`\b(?:href|src)="[^"]*"`)

// RenderHTML sets each message's HTML: the template rendered once, then
// filled in per recipient so that a value is always literal text, never
// Markdown or HTML. msgs must be PrepMails(cfg, template)'s result.
//
// goldmark percent-encodes a %KEY% in a URL only partly (%25FN%25 but
// %EA%25), so each placeholder is swapped for a token that it passes
// unchanged everywhere: "-<nonce><n>-". Like '%', '-' is punctuation, so
// emphasis parses the same, and it is never escaped. In href/src a value is
// percent-encoded (see fillURL), so it can only be part of a URL, never a
// whole URL, a scheme or an extra query parameter; elsewhere it is
// HTML-escaped.
func RenderHTML(cfg *config.MailConfig, template string, msgs []Message) error {
	if len(msgs) != len(cfg.Recipients) {
		return fmt.Errorf("cannot render the Markdown: %d messages for %d recipients", len(msgs), len(cfg.Recipients))
	}
	prefix := "-" + rand.Text()
	tokens := map[string]string{} // placeholder -> token
	tmpl := rePlaceholder.ReplaceAllStringFunc(template, func(p string) string {
		if _, ok := tokens[p]; !ok {
			tokens[p] = prefix + strconv.Itoa(len(tokens)) + "-"
		}
		return tokens[p]
	})
	var buf bytes.Buffer
	if err := md.Convert([]byte(tmpl), &buf); err != nil {
		return fmt.Errorf("cannot render the Markdown: %w", err)
	}
	rendered := buf.String()
	for _, p := range slices.Sorted(maps.Keys(tokens)) {
		if !strings.Contains(rendered, tokens[p]) {
			return fmt.Errorf("placeholder %s is not in the HTML (inside raw HTML or a dropped link?)", p)
		}
	}

	for i, r := range cfg.Recipients {
		vals := placeholderValues(r)
		var text, path, query, frag []string
		for p, tok := range tokens {
			v, ok := vals[p]
			if !ok {
				return fmt.Errorf("recipient '%s': unresolved placeholder in body: %s", r.Email, p)
			}
			text = append(text, tok, html.EscapeString(v))
			path = append(path, tok, strings.ReplaceAll(url.PathEscape(v), ":", "%3A"))
			query = append(query, tok, url.QueryEscape(v))
			frag = append(frag, tok, strings.ReplaceAll(url.QueryEscape(v), "+", "%20"))
		}
		f := urlFiller{strings.NewReplacer(path...), strings.NewReplacer(query...), strings.NewReplacer(frag...)}
		out := reURLAttr.ReplaceAllStringFunc(rendered, f.fillAttr)
		out = strings.NewReplacer(text...).Replace(out)
		// A token goldmark split or altered would be left unfilled.
		if strings.Contains(out, prefix) {
			return fmt.Errorf("recipient '%s': cannot fill in a placeholder in the HTML", r.Email)
		}
		msgs[i].HTML = out
	}
	return nil
}

// urlFiller fills the tokens of a URL with the values escaped for each part.
type urlFiller struct{ path, query, frag *strings.Replacer }

// fillAttr fills an href="…" or src="…" attribute.
func (f urlFiller) fillAttr(attr string) string {
	name, val, _ := strings.Cut(attr, `"`)
	return name + `"` + html.EscapeString(f.fillURL(html.UnescapeString(strings.TrimSuffix(val, `"`)))) + `"`
}

// fillURL fills a URL by part, split at the template's first '#' and the
// first '?' before it:
//   - path: url.PathEscape plus ':', so a value is no scheme and no extra
//     segment;
//   - query: url.QueryEscape, so '&' and '=' can't add a parameter;
//   - fragment: url.QueryEscape with spaces as %20, the stricter of the two
//     (it also escapes '&', '=', '+', '$' and '@').
func (f urlFiller) fillURL(u string) string {
	u, frag, hasFrag := strings.Cut(u, "#")
	u, query, hasQuery := strings.Cut(u, "?")
	out := f.path.Replace(u)
	if hasQuery {
		out += "?" + f.query.Replace(query)
	}
	if hasFrag {
		out += "#" + f.frag.Replace(frag)
	}
	return out
}
