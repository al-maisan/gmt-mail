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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSV(t *testing.T) {
	in := "\ufeffEmail, First ,LAST,Org,cc,cc_extra,attachments,attachments_extra\n" +
		"jd@example.com,John,\"Doe, Jr.\",\"EFF\nSF\",a@x.com; B <b@x.com>,,,\n" +
		"\n" +
		"mm@gmail.com,Mickey,,,,c@x.com,,x.pdf;y.pdf\n" +
		"ab@example.com,Alice,Brown,MIT,,,z.txt,\n"
	got, err := ParseCSV("people.csv", []byte(in), true)
	require.NoError(t, err)
	assert.Equal(t, []Recipient{
		{Email: "jd@example.com", First: "John", Last: "Doe, Jr.", Data: map[string]string{"ORG": "EFF\nSF"}, Cc: []string{"a@x.com", "B <b@x.com>"}},
		{Email: "mm@gmail.com", First: "Mickey", Data: map[string]string{"ORG": ""}, CcExtra: []string{"c@x.com"}, AttachmentsExtra: []string{"x.pdf", "y.pdf"}},
		{Email: "ab@example.com", First: "Alice", Last: "Brown", Data: map[string]string{"ORG": "MIT"}, Attachments: []string{"z.txt"}},
	}, got)
}

func TestParseCSVRejects(t *testing.T) {
	const h = "email,first\n"
	tests := []struct {
		name, input, want string
	}{
		{"empty file", "", "people.csv: file is empty"},
		{"only a BOM", "\ufeff", "people.csv: file is empty"},
		{"header only", h, "people.csv: no recipient rows found"},
		{"missing email column", "first\nA\n", "people.csv:1: missing required column \"email\""},
		{"missing first column", "email\na@b.com\n", "people.csv:1: missing required column \"first\""},
		{"duplicate column", "email,first,Email\na@b.com,A,c@d.com\n", "people.csv:1: duplicate column \"Email\""},
		{"invalid data key column", "email,first,my-key\na@b.com,A,x\n", "people.csv:1: column \"my-key\": invalid data key \"my-key\""},
		{"empty column name", "email,first,\na@b.com,A,x\n", "people.csv:1: column 3: empty column name"},
		{"blank column name", "email, ,first\na@b.com,x,A\n", "people.csv:1: column 2: empty column name"},
		{"reserved data key column", "email,first,fn\na@b.com,A,x\n", "reserved placeholder %FN%"},
		{"too many fields", h + "a@b.com,A\nc@d.com,C,x\n", "people.csv:3: wrong number of fields"},
		{"too few fields", h + "a@b.com\n", "people.csv:2: wrong number of fields"},
		{"bad quote", h + "a@b.com,\"A\n", "people.csv:2: extraneous or missing \" in quoted-field"},
		{"blank first", h + "a@b.com,A\nc@d.com, \n", "people.csv:3: recipient \"c@d.com\": 'first' must not be blank"},
		{"empty email", h + ",A\n", "people.csv:2: recipient \"\": email must be a bare address"},
		{"not a bare email", h + "A <a@b.com>,A\n", "email must be a bare address"},
		{"invalid email", h + "a@b,A\n", "people.csv:2: recipient \"a@b\": invalid email address"},
		{"duplicate recipient", h + "a@b.com,A\n\nA@B.com,B\n", "people.csv:4: duplicate recipient \"A@B.com\""},
		{"cc and cc_extra", "email,first,cc,cc_extra\na@b.com,A,c@d.com,e@f.com\n", "people.csv:2: recipient \"a@b.com\": set either 'cc' or 'cc_extra'"},
		{"attachments and attachments_extra", "email,first,attachments,attachments_extra\na@b.com,A,x,y\n", "set either 'attachments' or 'attachments_extra'"},
		{"invalid cc", "email,first,cc\na@b.com,A,c@d.com;bad\n", "people.csv:2: recipient \"a@b.com\": invalid address in 'cc': \"bad\""},
		{"invalid cc_extra", "email,first,cc_extra\na@b.com,A,c@d.com;\n", "invalid address in 'cc_extra': \"\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCSV("people.csv", []byte(tt.input), true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestParseCSVAttachments(t *testing.T) {
	for _, col := range []string{"attachments", "Attachments_Extra"} {
		in := "email,first," + col + "\na@b.com,A,x.pdf\n"
		_, err := ParseCSV("people.csv", []byte(in), false)
		assert.ErrorContains(t, err, "people.csv:1: column \""+col+"\": attachments from a CSV need -csv-attachments")
		got, err := ParseCSV("people.csv", []byte(in), true)
		require.NoError(t, err)
		assert.Equal(t, []string{"x.pdf"}, append(got[0].Attachments, got[0].AttachmentsExtra...))
	}
}

func TestParseGeneral(t *testing.T) {
	const general = "[general]\nfrom = \"a <a@a.com>\"\nsubject = \"s\"\ncc = [\"c@d.com\"]\n"
	cfg, err := ParseGeneral([]byte(general))
	require.NoError(t, err)
	assert.Equal(t, MailConfig{From: "a <a@a.com>", Subject: "s", Cc: []string{"c@d.com"}}, cfg)

	_, err = ParseGeneral([]byte(general + "[[recipients]]\nemail = \"a@b.com\"\nfirst = \"A\"\n"))
	assert.ErrorContains(t, err, "[[recipients]] must not be set when the recipients come from a CSV file")

	_, err = ParseGeneral([]byte("[general]\nsubject = \"s\"\n"))
	assert.ErrorContains(t, err, "missing required key 'from'")
}

func TestSampleCSVParses(t *testing.T) {
	cfg, err := Parse([]byte(SampleConfig("0.0.0")))
	require.NoError(t, err)
	got, err := ParseCSV("sample.csv", []byte(SampleCSV()), false)
	require.NoError(t, err)
	assert.Len(t, got, len(cfg.Recipients))
	for i, r := range got {
		assert.Equal(t, cfg.Recipients[i].Email, r.Email)
		assert.Equal(t, cfg.Recipients[i].Cc, r.Cc)
		assert.Equal(t, cfg.Recipients[i].CcExtra, r.CcExtra)
	}
}
