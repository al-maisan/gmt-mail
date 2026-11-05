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
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// csvKeys are the columns that map to [[recipients]] keys.
var csvKeys = map[string]struct{}{"email": {}, "first": {}, "last": {}, "cc": {}, "cc_extra": {}, "attachments": {}, "attachments_extra": {}}

// ParseCSV decodes the recipients from an RFC 4180 CSV file with a header row.
// The columns email, first and last and the ';'-separated lists cc, cc_extra,
// attachments and attachments_extra (header names are case-insensitive) map to
// the [[recipients]] keys; every other column becomes a data key. Each row gets
// the same checks as a [[recipients]] entry. name prefixes the error messages.
// The attachment columns are rejected unless attachments is set: a CSV from
// someone else must not pick which local files get sent.
func ParseCSV(name string, bs []byte, attachments bool) ([]Recipient, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(bs, []byte("\ufeff"))))
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: file is empty", name)
	}
	if err != nil {
		return nil, csvError(name, err)
	}
	cols, err := csvColumns(header, attachments)
	if err != nil {
		return nil, fmt.Errorf("%s:1: %w", name, err)
	}

	var recipients []Recipient
	seen := make(map[string]struct{})
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, csvError(name, err)
		}
		line, _ := r.FieldPos(0)
		e := tomlRecipient{Data: make(map[string]string)}
		for i, col := range cols {
			switch v := row[i]; col {
			case "email":
				e.Email = v
			case "first":
				e.First = v
			case "last":
				e.Last = v
			case "cc":
				e.Cc = splitList(v)
			case "cc_extra":
				e.CcExtra = splitList(v)
			case "attachments":
				e.Attachments = splitList(v)
			case "attachments_extra":
				e.AttachmentsExtra = splitList(v)
			default:
				e.Data[header[i]] = v
			}
		}
		rcpt, err := convertRecipient(e, seen)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, line, err)
		}
		recipients = append(recipients, rcpt)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("%s: no recipient rows found", name)
	}
	return recipients, nil
}

// csvColumns trims the header names in place and returns them lower-cased.
// It rejects empty or duplicate names, invalid data keys, a missing email or
// first, and the attachment columns unless attachments is set.
func csvColumns(header []string, attachments bool) ([]string, error) {
	cols := make([]string, len(header))
	seen := make(map[string]struct{}, len(header))
	for i, h := range header {
		header[i] = strings.TrimSpace(h)
		cols[i] = strings.ToLower(header[i])
		if cols[i] == "" {
			return nil, fmt.Errorf("column %d: empty column name", i+1)
		}
		if !attachments && (cols[i] == "attachments" || cols[i] == "attachments_extra") {
			return nil, fmt.Errorf("column %q: attachments from a CSV need -csv-attachments (a CSV from someone else could attach any of your files)", header[i])
		}
		if _, dup := seen[cols[i]]; dup {
			return nil, fmt.Errorf("duplicate column %q", header[i])
		}
		seen[cols[i]] = struct{}{}
		if _, key := csvKeys[cols[i]]; !key {
			if _, err := dataKey(header[i]); err != nil {
				return nil, fmt.Errorf("column %q: %w", header[i], err)
			}
		}
	}
	for _, c := range []string{"email", "first"} {
		if _, ok := seen[c]; !ok {
			return nil, fmt.Errorf("missing required column %q", c)
		}
	}
	return cols, nil
}

// csvError reports a CSV syntax error as "name:line: reason".
func csvError(name string, err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s:%d: %w", name, pe.Line, pe.Err)
	}
	return fmt.Errorf("%s: %w", name, err)
}

// splitList splits a ';'-separated cell into its trimmed values; an empty cell
// is not set.
func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	list := strings.Split(s, ";")
	for i, v := range list {
		list[i] = strings.TrimSpace(v)
	}
	return list
}
