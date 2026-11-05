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

// Package config handles parsing of gmt-mail TOML configuration files.
package config

import (
	_ "embed"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// tomlConfig mirrors the TOML file structure for decoding.
type tomlConfig struct {
	General    tomlGeneral     `toml:"general"    validate:"required"`
	Recipients []tomlRecipient `toml:"recipients" validate:"required,min=1,dive"`
}

// tomlGeneral holds the [general] section fields.
type tomlGeneral struct {
	From        string   `toml:"from"        validate:"required"`
	Subject     string   `toml:"subject"     validate:"required"`
	ReplyTo     string   `toml:"reply_to"`
	Cc          []string `toml:"cc"`
	Attachments []string `toml:"attachments"`
}

// tomlRecipient holds a single [[recipients]] entry.
type tomlRecipient struct {
	Email            string            `toml:"email"             validate:"required,email"`
	First            string            `toml:"first"             validate:"required"`
	Last             string            `toml:"last"`
	Data             map[string]string `toml:"data"`
	Cc               []string          `toml:"cc"`
	CcExtra          []string          `toml:"cc_extra"`
	Attachments      []string          `toml:"attachments"`
	AttachmentsExtra []string          `toml:"attachments_extra"`
}

// Recipient holds a parsed recipient entry from the config file.
type Recipient struct {
	Email            string
	First            string
	Last             string
	Data             map[string]string
	Cc               []string // replaces global Cc
	CcExtra          []string // appends to global Cc
	Attachments      []string // replaces global attachments
	AttachmentsExtra []string // appends to global attachments
}

// MailConfig holds the fully parsed configuration for a mailing run.
type MailConfig struct {
	From        string
	ReplyTo     string
	Cc          []string
	Subject     string
	Recipients  []Recipient
	Attachments []string
}

// Parse decodes TOML-formatted configuration bytes into a MailConfig.
func Parse(bs []byte) (MailConfig, error) {
	tc, err := parseGeneral(bs, false)
	if err != nil {
		return MailConfig{}, err
	}
	recipients, err := convertRecipients(tc.Recipients)
	if err != nil {
		return MailConfig{}, err
	}
	cfg := newMailConfig(tc.General)
	cfg.Recipients = recipients
	return cfg, nil
}

// ParseGeneral decodes a config whose recipients come from a CSV file (see
// ParseCSV): it must have no [[recipients]], and the result has none.
func ParseGeneral(bs []byte) (MailConfig, error) {
	tc, err := parseGeneral(bs, true)
	if err != nil {
		return MailConfig{}, err
	}
	return newMailConfig(tc.General), nil
}

// parseGeneral decodes and validates bs; the [[recipients]] are only checked
// against the struct tags, or must be absent if csv is set.
func parseGeneral(bs []byte, csv bool) (tomlConfig, error) {
	var tc tomlConfig
	md, err := toml.Decode(string(bs), &tc)
	if err != nil {
		return tc, fmt.Errorf("TOML syntax error: %w", err)
	}
	// A misspelled key (e.g. "atachments") would otherwise be silently dropped.
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return tc, fmt.Errorf("unknown key(s): %s", strings.Join(keys, ", "))
	}

	if csv {
		if len(tc.Recipients) > 0 {
			return tc, fmt.Errorf("[[recipients]] must not be set when the recipients come from a CSV file")
		}
		err = validate.StructExcept(tc, "Recipients")
	} else {
		err = validate.Struct(tc)
	}
	if err != nil {
		return tc, formatValidationError(err)
	}

	if strings.TrimSpace(tc.General.Subject) == "" {
		return tc, fmt.Errorf("subject must not be empty or whitespace")
	}

	return tc, checkGeneralAddresses(tc.General)
}

// newMailConfig returns a MailConfig with the [general] values and no recipients.
func newMailConfig(g tomlGeneral) MailConfig {
	return MailConfig{
		From:        g.From,
		Subject:     g.Subject,
		ReplyTo:     g.ReplyTo,
		Cc:          g.Cc,
		Attachments: g.Attachments,
	}
}

// formatValidationError converts validator errors into user-friendly messages.
func formatValidationError(err error) error {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}
	var msgs []string
	for _, fe := range ve {
		switch fe.StructNamespace() {
		case "tomlConfig.General.From":
			msgs = append(msgs, "missing required key 'from' in [general]")
		case "tomlConfig.General.Subject":
			msgs = append(msgs, "missing required key 'subject' in [general]")
		case "tomlConfig.Recipients":
			msgs = append(msgs, "no [[recipients]] entries found")
		default:
			field := fe.StructNamespace()
			if fe.Tag() == "required" {
				msgs = append(msgs, fmt.Sprintf("missing required field '%s'", field))
			} else if fe.Tag() == "email" {
				msgs = append(msgs, fmt.Sprintf("invalid email address in '%s': %q", field, fe.Value()))
			} else {
				msgs = append(msgs, fmt.Sprintf("validation failed for '%s': %s", field, fe.Tag()))
			}
		}
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}

// reservedDataKeys are the placeholder keys owned by the template engine
// (%EA%, %FN%, %LN%). A recipient data key that folds to one of these would be
// silently shadowed during substitution, so it is rejected at parse time.
var reservedDataKeys = map[string]struct{}{"EA": {}, "FN": {}, "LN": {}}

// reDataKey matches valid data keys; upper-cased they form a %KEY% placeholder
// that the template engine can detect when it is left unresolved.
var reDataKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// checkAddress parses s the same way the mailer does at send time, so a bad
// address is reported by -validate instead of failing every send.
func checkAddress(field, s string) error {
	if _, err := mail.ParseAddress(s); err != nil {
		return fmt.Errorf("invalid address in '%s': %q: %w", field, s, err)
	}
	return nil
}

// checkAddresses runs checkAddress on every entry of list.
func checkAddresses(field string, list []string) error {
	for _, s := range list {
		if err := checkAddress(field, s); err != nil {
			return err
		}
	}
	return nil
}

// checkGeneralAddresses validates the [general] from, reply_to and cc values.
func checkGeneralAddresses(g tomlGeneral) error {
	if err := checkAddress("from", g.From); err != nil {
		return err
	}
	if g.ReplyTo != "" {
		if err := checkAddress("reply_to", g.ReplyTo); err != nil {
			return err
		}
	}
	return checkAddresses("cc", g.Cc)
}

// checkRecipient validates a recipient's addresses and rejects ambiguous
// override combinations (a replace list together with its _extra list).
func checkRecipient(e tomlRecipient) error {
	if a, err := mail.ParseAddress(e.Email); err != nil || a.Name != "" || a.Address != e.Email {
		return fmt.Errorf("recipient %q: email must be a bare address", e.Email)
	}
	// Parse checks this via the struct tag; ParseCSV relies on it here.
	if validate.Var(e.Email, "email") != nil {
		return fmt.Errorf("recipient %q: invalid email address", e.Email)
	}
	// "required" accepts " ", which would give an empty name and greeting.
	if strings.TrimSpace(e.First) == "" {
		return fmt.Errorf("recipient %q: 'first' must not be blank", e.Email)
	}
	if len(e.Cc) > 0 && len(e.CcExtra) > 0 {
		return fmt.Errorf("recipient %q: set either 'cc' or 'cc_extra', not both", e.Email)
	}
	if len(e.Attachments) > 0 && len(e.AttachmentsExtra) > 0 {
		return fmt.Errorf("recipient %q: set either 'attachments' or 'attachments_extra', not both", e.Email)
	}
	if err := checkAddresses("cc", e.Cc); err != nil {
		return fmt.Errorf("recipient %q: %w", e.Email, err)
	}
	if err := checkAddresses("cc_extra", e.CcExtra); err != nil {
		return fmt.Errorf("recipient %q: %w", e.Email, err)
	}
	return nil
}

// convertRecipients transforms TOML recipient entries into Recipient structs.
func convertRecipients(entries []tomlRecipient) ([]Recipient, error) {
	recipients := make([]Recipient, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		r, err := convertRecipient(e, seen)
		if err != nil {
			return nil, err
		}
		recipients = append(recipients, r)
	}
	return recipients, nil
}

// convertRecipient checks e and converts it into a Recipient. Data keys are
// upper-cased to match %KEY% placeholders; it returns an error if two keys
// collide after folding, since that would nondeterministically pick a value.
// Duplicate recipient addresses (tracked in seen) are rejected to avoid double
// sends.
func convertRecipient(e tomlRecipient, seen map[string]struct{}) (Recipient, error) {
	if err := checkRecipient(e); err != nil {
		return Recipient{}, err
	}
	addr := strings.ToLower(e.Email)
	if _, dup := seen[addr]; dup {
		return Recipient{}, fmt.Errorf("duplicate recipient %q", e.Email)
	}
	seen[addr] = struct{}{}

	data := make(map[string]string, len(e.Data))
	for k, v := range e.Data {
		key, err := dataKey(k)
		if err != nil {
			return Recipient{}, fmt.Errorf("recipient %q: %w", e.Email, err)
		}
		if _, ok := data[key]; ok {
			return Recipient{}, fmt.Errorf("recipient %q: data keys collide after upper-casing to %q", e.Email, key)
		}
		data[key] = v
	}

	return Recipient{
		Email:            e.Email,
		First:            e.First,
		Last:             e.Last,
		Data:             data,
		Cc:               e.Cc,
		CcExtra:          e.CcExtra,
		Attachments:      e.Attachments,
		AttachmentsExtra: e.AttachmentsExtra,
	}, nil
}

// dataKey returns k upper-cased, or an error if k is malformed or collides
// with a reserved placeholder (whose value it would silently shadow).
func dataKey(k string) (string, error) {
	if !reDataKey.MatchString(k) {
		return "", fmt.Errorf("invalid data key %q: must start with a letter and contain only letters, digits and '_'", k)
	}
	key := strings.ToUpper(k)
	if _, ok := reservedDataKeys[key]; ok {
		return "", fmt.Errorf("data key %q collides with reserved placeholder %%%s%%", k, key)
	}
	return key, nil
}

//go:embed samples/config.toml
var sampleConfigContent string

//go:embed samples/recipients.csv
var sampleCSVContent string

//go:embed samples/template.eml
var sampleTemplateContent string

// SampleConfig returns a commented example configuration file.
func SampleConfig(version string) string {
	return "# gmt-mail version " + version + "\n" + sampleConfigContent
}

// SampleCSV returns an example recipients CSV file (see ParseCSV).
func SampleCSV() string {
	return strings.TrimRight(sampleCSVContent, "\n")
}

// SampleTemplate returns an example email template demonstrating placeholder usage.
func SampleTemplate() string {
	return strings.TrimRight(sampleTemplateContent, "\n")
}
