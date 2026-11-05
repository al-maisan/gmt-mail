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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/al-maisan/gmt-mail/config"
	"github.com/al-maisan/gmt-mail/email"
	"github.com/joho/godotenv"
)

const (
	exitOK          = 0
	exitUsageError  = 1
	exitConfigError = 2
	exitSMTPError   = 3
	exitSendFailure = 4
)

var (
	// Set via -ldflags at build time
	appVersion = "dev"
	gitCommit  = "unknown"
	buildDate  = "unknown"
)

func version() string {
	v, c, d := appVersion, gitCommit, buildDate
	if bi, ok := debug.ReadBuildInfo(); ok && v == "dev" {
		v, c, d = fromBuildInfo(bi, v, c, d)
	}
	return v + "-" + c + " (" + d + ")"
}

// rePseudo matches a Go pseudo-version (vX.Y.Z-[pre.]yyyymmddhhmmss-commit),
// which `go build` in a clone and `go install …@commit` stamp.
var rePseudo = regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:.*\.)?(\d{14})-([0-9a-f]{12})(\+dirty)?$`)

// fromBuildInfo fills in the version, commit and date from the Go build info
// for builds without -ldflags (`go install …@version`, plain `go build`).
// A pseudo-version is not a release, so it keeps the version "dev" and only
// supplies the commit and date.
func fromBuildInfo(bi *debug.BuildInfo, v, c, d string) (string, string, string) {
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			c = s.Value[:min(7, len(s.Value))]
		case "vcs.time":
			d = s.Value[:min(10, len(s.Value))]
		}
	}
	mv := bi.Main.Version
	if m := rePseudo.FindStringSubmatch(mv); m != nil {
		ts := m[1]
		c, d = m[2][:7]+m[3], ts[:4]+"-"+ts[4:6]+"-"+ts[6:8]
	} else if mv != "" && mv != "(devel)" {
		v = strings.TrimPrefix(mv, "v")
		if base, ok := strings.CutSuffix(v, "+dirty"); ok {
			v, c = base, c+"+dirty"
		}
	}
	return v, c, d
}

func help() {
	_, _ = fmt.Fprintf(flag.CommandLine.Output(), "\n%s, version %s\nThis tool sends emails in bulk based on a template and a config file\n\n", filepath.Base(os.Args[0]), version())
	flag.PrintDefaults()
}

func requireFlag(value, name string) {
	if value == "" {
		log.Printf("Error: %s flag is required", name)
		flag.Usage()
		os.Exit(exitUsageError)
	}
}

func main() {
	log.SetFlags(0)

	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to load .env file: %v", err)
	}

	flag.Usage = help
	configPath := flag.String("config-path", "", "path to the config file")
	doDryRun := flag.Bool("dry-run", false, "show what would be done but execute no action")
	doValidate := flag.Bool("validate", false, "validate config and template without sending")
	templatePath := flag.String("template-path", "", "path to the template file")
	recipientsPath := flag.String("recipients-path", "", "path to a CSV file with the recipients (the config file then has no [[recipients]])")
	csvAttachments := flag.Bool("csv-attachments", false, "allow the attachments and attachments_extra columns in the -recipients-path CSV")
	doSampleConfig := flag.Bool("sample-config", false, "output sample configuration to stdout")
	doSampleTemplate := flag.Bool("sample-template", false, "output sample template to stdout")
	doSampleCSV := flag.Bool("sample-csv", false, "output sample recipients CSV to stdout")
	doVersion := flag.Bool("version", false, "print version and exit")
	delay := flag.Duration("delay", 0, "delay between emails (e.g., 1s, 500ms)")
	retries := flag.Int("retries", 1, "max retry attempts per failed send")
	retryDelay := flag.Duration("retry-delay", 2*time.Second, "backoff between retries")
	timeout := flag.Duration("timeout", 30*time.Second, "SMTP connect/send timeout, must be > 0 (covers the full attachment upload)")

	flag.Parse()

	if actionFlagCount(*doVersion, *doSampleConfig, *doSampleTemplate, *doSampleCSV, *doValidate, *doDryRun) > 1 {
		log.Printf("Warning: multiple action flags set; only the first in precedence order takes effect")
	}

	if *doVersion {
		fmt.Println(version())
		os.Exit(exitOK)
	}
	if *doSampleConfig {
		fmt.Println(config.SampleConfig(version()))
		os.Exit(exitOK)
	}
	if *doSampleTemplate {
		fmt.Println(config.SampleTemplate())
		os.Exit(exitOK)
	}
	if *doSampleCSV {
		fmt.Println(config.SampleCSV())
		os.Exit(exitOK)
	}

	requireFlag(*configPath, "-config-path")
	requireFlag(*templatePath, "-template-path")

	if *retries < 0 {
		log.Printf("Error: -retries must be >= 0")
		flag.Usage()
		os.Exit(exitUsageError)
	}
	if *delay < 0 || *retryDelay < 0 {
		log.Printf("Error: -delay and -retry-delay must be >= 0")
		flag.Usage()
		os.Exit(exitUsageError)
	}
	if *timeout <= 0 {
		log.Printf("Error: -timeout must be > 0")
		flag.Usage()
		os.Exit(exitUsageError)
	}

	cfg, err := loadConfig(*configPath, *recipientsPath, *csvAttachments)
	if err != nil {
		log.Printf("Error: %v", err)
		os.Exit(exitConfigError)
	}

	msgs, err := prepMails(&cfg, *templatePath)
	if err != nil {
		log.Printf("Error: %v", err)
		os.Exit(exitConfigError)
	}

	if err := email.CheckAttachments(msgs); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(exitConfigError)
	}

	if *doValidate {
		fmt.Printf("Config and template are valid: %d recipient(s)\n", len(msgs))
		os.Exit(exitOK)
	}

	if *doDryRun {
		printDryRun(msgs)
		os.Exit(exitOK)
	}

	creds, err := email.LoadSMTPCredentials()
	if err != nil {
		log.Printf("SMTP configuration error: %v", err)
		os.Exit(exitSMTPError)
	}

	sender, err := email.NewSMTPSender(creds, *timeout)
	if err != nil {
		log.Printf("SMTP error: %v", err)
		os.Exit(exitSMTPError)
	}

	opts := email.SendOptions{
		Delay:      *delay,
		Retries:    *retries,
		RetryDelay: *retryDelay,
	}
	batch := email.NewBatchSender(os.Stdout, sender, cfg, opts)
	fmt.Println("\nSending emails now..")
	result := batch.SendAll(msgs)

	// Close explicitly (not via defer) so the graceful SMTP QUIT runs even on
	// the exitSendFailure path below — os.Exit does not run deferred calls.
	if cerr := sender.Close(); cerr != nil {
		log.Printf("Warning: failed to close SMTP connection: %v", cerr)
	}

	fmt.Printf("\nDone: %d sent, %d failed, %d total\n", result.Sent, result.Failed, result.Sent+result.Failed)

	if result.Failed > 0 {
		os.Exit(exitSendFailure)
	}
}

// actionFlagCount returns how many mutually-exclusive action flags are set.
func actionFlagCount(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}

// loadConfig reads the config file and, if recipientsPath is set, takes the
// recipients from that CSV file instead of [[recipients]]; csvAttachments
// allows its attachment columns.
func loadConfig(path, recipientsPath string, csvAttachments bool) (config.MailConfig, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return config.MailConfig{}, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	parse := config.Parse
	if recipientsPath != "" {
		parse = config.ParseGeneral
	}
	cfg, err := parse(bs)
	if err != nil {
		return config.MailConfig{}, fmt.Errorf("invalid config file %q: %w", path, err)
	}
	if recipientsPath == "" {
		return cfg, nil
	}

	bs, err = os.ReadFile(recipientsPath)
	if err != nil {
		return config.MailConfig{}, fmt.Errorf("failed to read recipients file %q: %w", recipientsPath, err)
	}
	if cfg.Recipients, err = config.ParseCSV(recipientsPath, bs, csvAttachments); err != nil {
		return config.MailConfig{}, fmt.Errorf("invalid recipients file: %w", err)
	}

	return cfg, nil
}

func prepMails(cfg *config.MailConfig, templatePath string) ([]email.Message, error) {
	bs, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %q: %w", templatePath, err)
	}
	if len(strings.TrimSpace(string(bs))) == 0 {
		return nil, fmt.Errorf("template file %q is empty", templatePath)
	}

	msgs, err := email.PrepMails(cfg, string(bs))
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("no recipients found in config file")
	}

	return msgs, nil
}

func printDryRun(msgs []email.Message) {
	for _, m := range msgs {
		fmt.Printf("--\n\"%s\" <%s>\n", m.Name, m.Address)
		if len(m.Cc) > 0 {
			fmt.Printf("Cc: %s\n", strings.Join(m.Cc, ", "))
		}
		fmt.Printf("Subject: %s\n", m.Subject)
		if len(m.Attachments) > 0 {
			fmt.Printf("Attachments: %s\n", strings.Join(m.Attachments, ", "))
		}
		fmt.Printf("%s\n", m.Body)
	}
}
