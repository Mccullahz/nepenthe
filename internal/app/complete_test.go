package app

import (
	"reflect"
	"testing"
)

func TestPrefixMatches(t *testing.T) {
	cands := []string{"daily", "projects", "projects/cli", "projects/web"}
	got := prefixMatches("pro", cands)
	want := []string{"projects", "projects/cli", "projects/web"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("prefixMatches(pro) = %v, want %v", got, want)
	}
	// Case-insensitive, and an empty prefix matches everything.
	if got := prefixMatches("PROJECTS/W", cands); !reflect.DeepEqual(got, []string{"projects/web"}) {
		t.Errorf("case-insensitive match = %v", got)
	}
	if got := prefixMatches("", cands); len(got) != len(cands) {
		t.Errorf("empty prefix should match all, got %v", got)
	}
	if got := prefixMatches("zzz", cands); got != nil {
		t.Errorf("no-match should be nil, got %v", got)
	}
}

func TestFirstSegment(t *testing.T) {
	for in, want := range map[string]string{
		"index.md":            "",
		"projects/a.md":       "projects",
		"projects/web/api.md": "projects",
	} {
		if got := firstSegment(in); got != want {
			t.Errorf("firstSegment(%q) = %q, want %q", in, got, want)
		}
	}
}
