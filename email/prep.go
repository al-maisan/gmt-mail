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

// Package email handles template substitution, per-recipient mail preparation,
// and SMTP delivery.
package email

import (
	"fmt"
	"net/mail"
	"regexp"
	"slices"
	"strings"

	"github.com/al-maisan/gmt-mail/config"
)

// rePlaceholder matches template variables in the form %KEY% where KEY is
// one or more uppercase letters, digits, or underscores.
var rePlaceholder = regexp.MustCompile(`%[A-Z][A-Z0-9_]*%`)

const (
	placeholderEmail     = "%EA%"
	placeholderFirstName = "%FN%"
	placeholderLastName  = "%LN%"
)

// Message holds a fully prepared email ready for sending.
type Message struct {
	Name        string
	Address     string
	Body        string
	Subject     string
	Cc          []string
	Attachments []string
}

// substituteVariables replaces placeholder tokens (%FN%, %LN%, %EA%, and
// any custom keys from recipient.Data) in text with their values.
func substituteVariables(recipient config.Recipient, text string) string {
	pairs := []string{
		placeholderEmail, recipient.Email,
		placeholderFirstName, recipient.First,
		placeholderLastName, recipient.Last,
	}
	for k, v := range recipient.Data {
		pairs = append(pairs, fmt.Sprintf("%%%s%%", k), v)
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// placeholderKey returns the KEY inside a "%KEY%" placeholder constant.
func placeholderKey(p string) string { return p[1 : len(p)-1] }

// availableKeys returns the set of placeholder keys resolvable for a recipient:
// the reserved keys (EA/FN/LN) plus the recipient's upper-cased data keys.
func availableKeys(r config.Recipient) map[string]struct{} {
	keys := make(map[string]struct{}, len(r.Data)+3)
	keys[placeholderKey(placeholderEmail)] = struct{}{}
	keys[placeholderKey(placeholderFirstName)] = struct{}{}
	keys[placeholderKey(placeholderLastName)] = struct{}{}
	for k := range r.Data {
		keys[k] = struct{}{}
	}
	return keys
}

// unresolvedPlaceholders returns the distinct %KEY% tokens in the ORIGINAL
// (pre-substitution) text whose KEY is not resolvable. Scanning the original
// text rather than the substituted result avoids misreporting substituted
// values that merely look like placeholders (e.g. a data value "50%OFF%deal").
func unresolvedPlaceholders(text string, keys map[string]struct{}) []string {
	var missing []string
	seen := make(map[string]struct{})
	for _, m := range rePlaceholder.FindAllString(text, -1) {
		if _, ok := keys[placeholderKey(m)]; ok {
			continue
		}
		if _, dup := seen[m]; dup {
			continue
		}
		seen[m] = struct{}{}
		missing = append(missing, m)
	}
	return missing
}

// PrepMails generates a Message for each recipient by substituting template
// variables and resolving per-recipient Cc and attachment overrides.
// Returns an error if any placeholders remain unresolved, or if a subject or
// body is blank after substitution (e.g. subject "%LN%" and no last name).
func PrepMails(cfg *config.MailConfig, template string) ([]Message, error) {
	var errs []string
	mails := make([]Message, 0, len(cfg.Recipients))
	for _, recipient := range cfg.Recipients {
		cc := dedupeCc(resolveOverride(cfg.Cc, recipient.Cc, recipient.CcExtra), recipient.Email)
		attachments := resolveOverride(cfg.Attachments, recipient.Attachments, recipient.AttachmentsExtra)

		keys := availableKeys(recipient)
		if unresolved := unresolvedPlaceholders(cfg.Subject, keys); len(unresolved) > 0 {
			errs = append(errs, fmt.Sprintf("recipient '%s': unresolved placeholder(s) in subject: %s", recipient.Email, strings.Join(unresolved, ", ")))
		}
		if unresolved := unresolvedPlaceholders(template, keys); len(unresolved) > 0 {
			errs = append(errs, fmt.Sprintf("recipient '%s': unresolved placeholder(s) in body: %s", recipient.Email, strings.Join(unresolved, ", ")))
		}

		subject := substituteVariables(recipient, cfg.Subject)
		body := substituteVariables(recipient, template)
		if strings.TrimSpace(subject) == "" {
			errs = append(errs, fmt.Sprintf("recipient '%s': subject is blank after substitution", recipient.Email))
		}
		if strings.TrimSpace(body) == "" {
			errs = append(errs, fmt.Sprintf("recipient '%s': body is blank after substitution", recipient.Email))
		}

		name := strings.TrimSpace(recipient.First + " " + recipient.Last)
		mails = append(mails, Message{
			Name:        name,
			Address:     recipient.Email,
			Subject:     subject,
			Body:        body,
			Cc:          cc,
			Attachments: attachments,
		})
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("cannot prepare the messages:\n  %s", strings.Join(errs, "\n  "))
	}
	return mails, nil
}

// resolveOverride returns the effective list for a field (Cc or attachments).
// If replace is set, it replaces global. If extra is set, it appends to global.
// Otherwise, global is returned as-is.
func resolveOverride(global, replace, extra []string) []string {
	if len(replace) > 0 {
		return replace
	}
	if len(extra) > 0 {
		return append(slices.Clone(global), extra...)
	}
	return slices.Clone(global)
}

// dedupeCc drops repeated Cc addresses and the To address itself, comparing the
// bare addresses case-insensitively; the first spelling of each is kept.
// Otherwise the Cc header lists an address twice and the recipient is Cc'd on
// their own mail. The addresses were checked at parse time.
func dedupeCc(cc []string, to string) []string {
	seen := map[string]struct{}{strings.ToLower(to): {}}
	var out []string
	for _, c := range cc {
		key := strings.ToLower(c)
		if a, err := mail.ParseAddress(c); err == nil {
			key = strings.ToLower(a.Address)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}
