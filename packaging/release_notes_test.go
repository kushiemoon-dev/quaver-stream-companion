package packaging

import (
	"regexp"
	"strings"
	"testing"
)

func TestReleaseNotesComeFromTheTagFileWhenItExists(t *testing.T) {
	rel := jobBlock(t, read(t, ".github/workflows/release.yml"), "release")

	if !strings.Contains(rel, "actions/checkout@") {
		t.Error("the release job must check out the repository to read docs/releases")
	}
	if !strings.Contains(rel, "TAG: ${{ github.ref_name }}") {
		t.Error("the tag must reach the shell through the TAG env variable")
	}
	if strings.Contains(strings.ReplaceAll(rel, "TAG: ${{ github.ref_name }}", ""), "${{ github.ref_name }}") {
		t.Error("github.ref_name may only be used in an env block, never inside a script")
	}
	for _, want := range []string{"docs/releases/", "--notes-file", "--generate-notes", "--draft", "--prerelease", "--verify-tag"} {
		if !strings.Contains(rel, want) {
			t.Errorf("the release job does not contain %q", want)
		}
	}

	check := strings.Index(rel, `[[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]]`)
	path := strings.Index(rel, "docs/releases/$TAG.md")
	if check < 0 {
		t.Fatal("the release job must validate TAG with the same regex as build-archive.sh")
	}
	if path < check {
		t.Error("the notes path must be built only after the tag is validated")
	}
	if !regexp.MustCompile(`(?s)-s "\$notes".*--notes-file "\$notes".*--generate-notes`).MatchString(rel) {
		t.Error("the notes file must be used when it exists and is not empty, with --generate-notes as the fallback")
	}
}
