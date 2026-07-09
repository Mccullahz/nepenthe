package noteview

import (
	"strings"
	"testing"

	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

func TestExtractLinksDocumentOrder(t *testing.T) {
	src := strings.Join([]string{
		"Intro [[alpha]] then [beta](beta.md) and [[gamma|Gee]].",
		"External [ext](https://example.com) and [mail](mailto:x@y.z) skipped.",
		"Anchor [self](#section) skipped, heading [[delta#Notes]].",
		"Inline `[[nope]]` stays raw.",
		"```",
		"fenced [[nofence]] and [x](x.md)",
		"```",
		"Tail [[omega]].",
	}, "\n")

	links := extractLinks(src)
	var got []string
	for _, l := range links {
		got = append(got, l.target)
	}
	want := []string{"alpha", "beta.md", "gamma", "delta", "omega"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("targets in order = %v, want %v", got, want)
	}

	if links[0].isWiki != true {
		t.Errorf("alpha should be a wikilink")
	}
	if links[1].isWiki != false || links[1].target != "beta.md" {
		t.Errorf("beta should be a markdown link, got %+v", links[1])
	}
	if links[2].alias != "Gee" {
		t.Errorf("gamma alias = %q, want Gee", links[2].alias)
	}
	if links[3].heading != "Notes" {
		t.Errorf("delta heading = %q, want Notes", links[3].heading)
	}
}

func TestExtractLinksSkipsFencesAndInline(t *testing.T) {
	src := "before [[real]]\n```\n[[incode]]\n```\nafter `[[inline]]` end"
	links := extractLinks(src)
	if len(links) != 1 || links[0].target != "real" {
		t.Fatalf("expected only [[real]], got %+v", links)
	}
}

func TestTransformSourceWikilinks(t *testing.T) {
	src := "See [[alpha|A]] and [[beta]].\n```\ncode [[gamma]]\n```"
	links := extractLinks(src)
	out := transformSource(src, links)

	if strings.Contains(out, "[[alpha") || strings.Contains(out, "[[beta") {
		t.Errorf("transform left raw wiki brackets: %q", out)
	}
	if !strings.Contains(out, "A") {
		t.Errorf("transform dropped alias display text: %q", out)
	}
	// Fenced wikilink must survive untouched.
	if !strings.Contains(out, "[[gamma]]") {
		t.Errorf("transform corrupted fenced code: %q", out)
	}
	// Sentinels must wrap the transformed display text.
	if !strings.ContainsRune(out, sentOpen) || !strings.ContainsRune(out, sentClose) {
		t.Errorf("transform missing sentinels: %q", out)
	}
}

func TestResolveLinks(t *testing.T) {
	v := &vault.Vault{Notes: map[string]*vault.Note{
		"a.md":         {Path: "a.md", Links: []string{"sub/alpha.md"}},
		"sub/alpha.md": {Path: "sub/alpha.md"},
		"beta.md":      {Path: "beta.md"},
	}}
	m := &Model{v: v, path: "a.md", current: -1}
	m.links = extractLinks("[[alpha]] and [beta](beta.md) and [[missing]]")
	m.resolveLinks()

	if got := m.links[0].resolved; got != "sub/alpha.md" {
		t.Errorf("wikilink alpha resolved = %q, want sub/alpha.md", got)
	}
	if got := m.links[1].resolved; got != "beta.md" {
		t.Errorf("markdown beta resolved = %q, want beta.md", got)
	}
	if got := m.links[2].resolved; got != "" {
		t.Errorf("missing link resolved = %q, want empty", got)
	}
}

func TestStemLookupShortestPath(t *testing.T) {
	v := &vault.Vault{Notes: map[string]*vault.Note{
		"deep/nested/x.md": {Path: "deep/nested/x.md"},
		"x.md":             {Path: "x.md"},
	}}
	m := &Model{v: v, path: "other.md"}
	if got := m.stemLookup("x"); got != "x.md" {
		t.Errorf("stemLookup tie-break = %q, want x.md (shallowest)", got)
	}
}
