package main

import (
	"strings"
	"testing"
)

func TestBuildVersionPrefersStamp(t *testing.T) {
	stamped := version
	t.Cleanup(func() { version = stamped })
	version = "v1.2.3"
	if got := buildVersion(); got != "v1.2.3" {
		t.Errorf("buildVersion() = %q, want v1.2.3", got)
	}
}

func TestBuildVersionUnstamped(t *testing.T) {
	stamped := version
	t.Cleanup(func() { version = stamped })
	version = ""
	if got := buildVersion(); !strings.HasPrefix(got, "dev") {
		t.Errorf("buildVersion() = %q, want a dev version", got)
	}
}
