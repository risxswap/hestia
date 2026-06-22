package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunAdminCommandHandlesMigrateVersion(t *testing.T) {
	var out bytes.Buffer

	if err := runAdminCommand([]string{"migrate", "version"}, &out); err != nil {
		t.Fatalf("run migrate version: %v", err)
	}

	if !strings.Contains(out.String(), "migrate version") {
		t.Fatalf("expected migrate version output, got %q", out.String())
	}
}

func TestRunAdminCommandRejectsUnsupportedMigrateCommand(t *testing.T) {
	var out bytes.Buffer

	err := runAdminCommand([]string{"migrate", "bad"}, &out)
	if err == nil {
		t.Fatal("expected unsupported migrate command error")
	}
	if !strings.Contains(err.Error(), "unsupported migrate command") {
		t.Fatalf("unexpected error %q", err.Error())
	}
}
