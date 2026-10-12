package version

import (
	"strings"
	"testing"
)

func TestStringContainsNameAndVersion(t *testing.T) {
	got := String()
	if !strings.HasPrefix(got, "quaver-stream-companion ") {
		t.Fatalf("String() = %q, want prefix %q", got, "quaver-stream-companion ")
	}
	if !strings.HasSuffix(got, Version) {
		t.Fatalf("String() = %q, want suffix %q", got, Version)
	}
}
