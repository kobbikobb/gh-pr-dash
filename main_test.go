package main

import (
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	t.Run("should default the interval to one minute", func(t *testing.T) {
		o, ok := parseArgs(nil)

		if !ok || o.interval != time.Minute {
			t.Errorf("ok=%v interval=%v", ok, o.interval)
		}
	})

	t.Run("should parse a custom --interval", func(t *testing.T) {
		o, ok := parseArgs([]string{"--interval", "30s"})

		if !ok || o.interval != 30*time.Second {
			t.Errorf("ok=%v interval=%v", ok, o.interval)
		}
	})

	t.Run("should reject an invalid --interval", func(t *testing.T) {
		_, ok := parseArgs([]string{"--interval", "nope"})

		if ok {
			t.Error("expected rejection")
		}
	})

	t.Run("should reject a non-positive max", func(t *testing.T) {
		_, ok := parseArgs([]string{"-5"})

		if ok {
			t.Error("expected rejection")
		}
	})

	t.Run("should reject --watch combined with --json", func(t *testing.T) {
		_, ok := parseArgs([]string{"--watch", "--json"})

		if ok {
			t.Error("expected rejection")
		}
	})

	t.Run("should parse --org", func(t *testing.T) {
		o, ok := parseArgs([]string{"--org", "my-org"})

		if !ok || o.org != "my-org" {
			t.Errorf("ok=%v org=%q", ok, o.org)
		}
	})

	t.Run("should parse --repo", func(t *testing.T) {
		o, ok := parseArgs([]string{"--repo", "owner/repo"})

		if !ok || o.repo != "owner/repo" {
			t.Errorf("ok=%v repo=%q", ok, o.repo)
		}
	})

	t.Run("should reject on --help", func(t *testing.T) {
		for _, flag := range []string{"-h", "--help"} {
			_, ok := parseArgs([]string{flag})

			if ok {
				t.Errorf("expected rejection for %s", flag)
			}
		}
	})
}
