package vault

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeFile writes content to root/rel, creating parent directories.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func openVault(t *testing.T) (*Vault, string) {
	t.Helper()
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return v, root
}

// --- Scanning ---------------------------------------------------------

func TestRescanSkipsHiddenAndNonMarkdown(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "note.md", "# Note\n")
	writeFile(t, root, "sub/child.md", "# Child\n")
	writeFile(t, root, "not-markdown.txt", "hello")
	writeFile(t, root, ".hidden.md", "# Hidden file\n")
	writeFile(t, root, ".nepenthe/config.md", "# Should be skipped\n")
	writeFile(t, root, ".git/HEAD.md", "# Should be skipped\n")

	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	if len(v.Notes) != 2 {
		var got []string
		for p := range v.Notes {
			got = append(got, p)
		}
		t.Fatalf("expected 2 notes, got %d: %v", len(v.Notes), got)
	}
	if _, ok := v.Notes["note.md"]; !ok {
		t.Errorf("missing note.md")
	}
	if _, ok := v.Notes["sub/child.md"]; !ok {
		t.Errorf("missing sub/child.md")
	}
}

func TestRescanIsIdempotent(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "# A\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan 1: %v", err)
	}
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan 2: %v", err)
	}
	if len(v.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(v.Notes))
	}
}

func TestNoteCaseInsensitiveExtension(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "upper.MD", "# Upper\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if _, ok := v.Notes["upper.MD"]; !ok {
		t.Errorf("expected upper.MD to be indexed")
	}
}

// --- Title / tags extraction -------------------------------------------

func TestTitleFromFrontmatter(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "note.md", "---\ntitle: Custom Title\ntags: [a, b]\n---\n# Ignored Heading\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["note.md"]
	if n.Title != "Custom Title" {
		t.Errorf("Title = %q, want %q", n.Title, "Custom Title")
	}
	if !equalUnordered(n.Tags, []string{"a", "b"}) {
		t.Errorf("Tags = %v, want [a b]", n.Tags)
	}
}

func TestTitleFromH1(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "note.md", "Some preamble\n\n# The Real Title\n\nBody text.\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["note.md"]
	if n.Title != "The Real Title" {
		t.Errorf("Title = %q, want %q", n.Title, "The Real Title")
	}
}

func TestTitleFallsBackToFilenameStem(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "my-note-name.md", "no heading here, just text\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["my-note-name.md"]
	if n.Title != "my-note-name" {
		t.Errorf("Title = %q, want %q", n.Title, "my-note-name")
	}
}

func TestBlockStyleFrontmatterTags(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "note.md", "---\ntitle: Block Tags\ntags:\n  - alpha\n  - beta/gamma\n---\nBody\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["note.md"]
	if !equalUnordered(n.Tags, []string{"alpha", "beta/gamma"}) {
		t.Errorf("Tags = %v, want [alpha beta/gamma]", n.Tags)
	}
}

func TestInlineTagsInBody(t *testing.T) {
	v, root := openVault(t)
	content := "# Note\n\nThis mentions #project and #area/work but not a heading:\n\n## Not a #tag-in-heading-but-still-a-tag\n\n```\n#not-a-tag inside fence\n```\n\nAlso inline `#not-a-tag` code.\n"
	writeFile(t, root, "note.md", content)
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["note.md"]
	want := []string{"project", "area/work", "tag-in-heading-but-still-a-tag"}
	if !equalUnordered(n.Tags, want) {
		t.Errorf("Tags = %v, want (unordered) %v", n.Tags, want)
	}
	for _, tag := range n.Tags {
		if tag == "not-a-tag" {
			t.Errorf("tag extraction leaked into fenced/inline code: %v", n.Tags)
		}
	}
}

func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// --- Link resolution -----------------------------------------------------

func TestWikilinkResolution(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "# A\n\nSee [[b]] and [[b|My Alias]] and [[b#Some Heading]] and [[b#Heading|Alias Too]].\n")
	writeFile(t, root, "b.md", "# B\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["a.md"]
	if len(n.Links) != 1 || n.Links[0] != "b.md" {
		t.Errorf("Links = %v, want [b.md] (deduped)", n.Links)
	}
}

func TestMarkdownLinkResolution(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "# A\n\n[link text](b.md) and [external](https://example.com) and [mail](mailto:x@y.com) and [abs](/etc/passwd.md).\n")
	writeFile(t, root, "b.md", "# B\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["a.md"]
	if len(n.Links) != 1 || n.Links[0] != "b.md" {
		t.Errorf("Links = %v, want [b.md]", n.Links)
	}
}

func TestVaultRelativePathResolution(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "dir1/a.md", "[[dir2/b]]\n")
	writeFile(t, root, "dir2/b.md", "# B\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["dir1/a.md"]
	if len(n.Links) != 1 || n.Links[0] != "dir2/b.md" {
		t.Errorf("Links = %v, want [dir2/b.md]", n.Links)
	}
}

func TestStemAmbiguityPrefersShortestPath(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "[[dup]]\n")
	writeFile(t, root, "dup.md", "# Short\n")
	writeFile(t, root, "deep/nested/dup.md", "# Long\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["a.md"]
	if len(n.Links) != 1 || n.Links[0] != "dup.md" {
		t.Errorf("Links = %v, want [dup.md] (shortest path wins)", n.Links)
	}
}

func TestStemAmbiguityLexicographicTiebreak(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "root.md", "[[dup]]\n")
	writeFile(t, root, "bdir/dup.md", "# B\n")
	writeFile(t, root, "adir/dup.md", "# A\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["root.md"]
	if len(n.Links) != 1 || n.Links[0] != "adir/dup.md" {
		t.Errorf("Links = %v, want [adir/dup.md] (lexicographic tiebreak)", n.Links)
	}
}

func TestNoSelfLinksAndUnresolvedDropped(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "[[a]] and [[does-not-exist]] and [[a|Alias To Self]]\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	n := v.Notes["a.md"]
	if len(n.Links) != 0 {
		t.Errorf("Links = %v, want none (self-link and unresolved dropped)", n.Links)
	}
}

// --- Graph ---------------------------------------------------------------

func TestGraphBaseFilteringAndDegree(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "projects/a.md", "[[projects/b]]\n")
	writeFile(t, root, "projects/b.md", "[[projects/a]]\n")
	writeFile(t, root, "journal/c.md", "# C\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	full := v.Graph("")
	if len(full.Nodes) != 3 {
		t.Fatalf("full graph nodes = %d, want 3", len(full.Nodes))
	}
	if len(full.Edges) != 2 {
		t.Fatalf("full graph edges = %d, want 2", len(full.Edges))
	}

	projects := v.Graph("projects")
	if len(projects.Nodes) != 2 {
		t.Fatalf("projects graph nodes = %d, want 2", len(projects.Nodes))
	}
	// a.md -> b.md and b.md -> a.md give each node one outgoing and one
	// incoming edge, for a total degree of 2.
	for _, node := range projects.Nodes {
		if node.Degree != 2 {
			t.Errorf("node %s degree = %d, want 2", node.Path, node.Degree)
		}
		if node.Base != "projects" {
			t.Errorf("node %s base = %q, want projects", node.Path, node.Base)
		}
	}

	// Deterministic node ordering by path.
	if projects.Nodes[0].Path != "projects/a.md" || projects.Nodes[1].Path != "projects/b.md" {
		t.Errorf("node ordering not sorted by path: %+v", projects.Nodes)
	}

	journal := v.Graph("journal")
	if len(journal.Nodes) != 1 || len(journal.Edges) != 0 {
		t.Errorf("journal graph = %+v, want 1 node 0 edges", journal)
	}
}

// --- Bases -----------------------------------------------------------------

func TestBasesCounts(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "root.md", "# Root\n")
	writeFile(t, root, "projects/a.md", "# A\n")
	writeFile(t, root, "projects/sub/b.md", "# B\n")
	writeFile(t, root, "journal/c.md", "# C\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	bases := v.Bases()
	if len(bases) != 3 {
		t.Fatalf("Bases = %+v, want 3 entries", bases)
	}
	if bases[0].Name != "" || bases[0].NoteCount != 4 {
		t.Errorf("root base = %+v, want {Name:\"\" NoteCount:4}", bases[0])
	}
	byName := map[string]Base{}
	for _, b := range bases {
		byName[b.Name] = b
	}
	if byName["journal"].NoteCount != 1 {
		t.Errorf("journal count = %d, want 1", byName["journal"].NoteCount)
	}
	if byName["projects"].NoteCount != 2 {
		t.Errorf("projects count = %d, want 2 (recursive)", byName["projects"].NoteCount)
	}
	// Sorted after the root base.
	if bases[1].Name != "journal" || bases[2].Name != "projects" {
		t.Errorf("bases not sorted: %+v", bases)
	}
}

// --- CRUD ------------------------------------------------------------------

func TestCreateWriteReadDelete(t *testing.T) {
	v, _ := openVault(t)
	if err := v.Create("new-note"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, ok := v.Notes["new-note.md"]; !ok {
		t.Fatalf("new-note.md not indexed after Create")
	}
	if err := v.Create("new-note"); err == nil {
		t.Errorf("Create should fail on existing note")
	}

	content, err := v.Read("new-note.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "# new-note") {
		t.Errorf("seed content = %q, want to contain a heading", content)
	}

	if err := v.Write("new-note.md", "# new-note\n\nUpdated.\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	if err := v.Delete("new-note.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := v.Notes["new-note.md"]; ok {
		t.Errorf("new-note.md still indexed after Delete")
	}
}

// --- Import ------------------------------------------------------------

func TestImportSingleFile(t *testing.T) {
	v, _ := openVault(t)
	srcDir := t.TempDir()
	writeFile(t, srcDir, "external.md", "# External\n")

	if err := v.Import(filepath.Join(srcDir, "external.md"), "imported"); err != nil {
		t.Fatalf("Import: %v", err)
	}
	if _, ok := v.Notes["imported/external.md"]; !ok {
		t.Fatalf("expected imported/external.md to be indexed")
	}
}

func TestImportRejectsNonMarkdownFile(t *testing.T) {
	v, _ := openVault(t)
	srcDir := t.TempDir()
	writeFile(t, srcDir, "notes.txt", "hello")

	if err := v.Import(filepath.Join(srcDir, "notes.txt"), ""); err == nil {
		t.Errorf("Import should reject non-markdown file")
	}
}

func TestImportDirectoryWithCollision(t *testing.T) {
	v, root := openVault(t)
	// Pre-existing note that will collide with the import.
	writeFile(t, root, "dest/existing.md", "# Already here\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	srcDir := t.TempDir()
	writeFile(t, srcDir, "existing.md", "# Colliding import\n")
	writeFile(t, srcDir, "fresh.md", "# Fresh\n")
	writeFile(t, srcDir, "assets/pic.png", "binarydata")
	writeFile(t, srcDir, ".hidden/skip.md", "# Skip\n")

	err := v.Import(srcDir, "dest")
	if err == nil {
		t.Fatalf("expected error listing collisions")
	}
	if !strings.Contains(err.Error(), "existing.md") {
		t.Errorf("error %q should mention the collision", err.Error())
	}

	if _, ok := v.Notes["dest/fresh.md"]; !ok {
		t.Errorf("dest/fresh.md should have been imported despite the collision")
	}
	// The pre-existing note must not have been overwritten.
	content, err := v.Read("dest/existing.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(content, "Already here") {
		t.Errorf("existing.md content = %q, was overwritten", content)
	}
	if _, err := os.Stat(filepath.Join(v.Root, "dest/assets/pic.png")); err != nil {
		t.Errorf("expected asset file to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v.Root, "dest/.hidden/skip.md")); err == nil {
		t.Errorf("hidden directory should have been skipped")
	}
}

// --- Export ------------------------------------------------------------

func TestExportSingleNote(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "note.md", "# Note\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	destDir := t.TempDir()
	if err := v.Export("note.md", destDir); err != nil {
		t.Fatalf("Export to dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "note.md")); err != nil {
		t.Errorf("expected exported file in dest dir: %v", err)
	}

	explicitTarget := filepath.Join(t.TempDir(), "renamed.md")
	if err := v.Export("note.md", explicitTarget); err != nil {
		t.Fatalf("Export to explicit path: %v", err)
	}
	if _, err := os.Stat(explicitTarget); err != nil {
		t.Errorf("expected exported file at explicit target: %v", err)
	}
}

func TestExportWholeVault(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "# A\n")
	writeFile(t, root, "sub/b.md", "# B\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	dest := filepath.Join(t.TempDir(), "export-out")
	if err := v.Export("", dest); err != nil {
		t.Fatalf("Export whole vault: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.md")); err != nil {
		t.Errorf("missing exported a.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "sub", "b.md")); err != nil {
		t.Errorf("missing exported sub/b.md: %v", err)
	}
}

func TestExportWholeVaultConflict(t *testing.T) {
	v, root := openVault(t)
	writeFile(t, root, "a.md", "# A new\n")
	if err := v.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	dest := t.TempDir()
	writeFile(t, dest, "a.md", "# A old\n")

	err := v.Export("", dest)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	content, rerr := os.ReadFile(filepath.Join(dest, "a.md"))
	if rerr != nil {
		t.Fatalf("read dest a.md: %v", rerr)
	}
	if !strings.Contains(string(content), "A old") {
		t.Errorf("existing export target was overwritten: %q", content)
	}
}
