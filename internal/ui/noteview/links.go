package noteview

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/mccullahz/nepenthe-cli/internal/vault"
)

// link is one outgoing reference found in a note's raw markdown, in
// document order. Wikilinks ([[target|alias]], [[target#heading]]) and
// relative markdown links ([alias](target.md)) are both captured;
// http(s)/mailto links are skipped.
type link struct {
	isWiki   bool
	target   string // link target, heading and alias stripped
	heading  string // wikilink #heading, if any
	alias    string // explicit display text, if any
	display  string // text shown in place of the raw link
	resolved string // vault-relative path, "" when unresolved
	start    int    // byte offset of the raw match in the source
	end      int    // byte offset just past the raw match
}

var (
	reWiki   = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)
	reMdLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)
	reInline = regexp.MustCompile("`[^`]*`")
)

// extractLinks returns every navigable link in src, in document order.
// Links inside fenced code blocks and inline code spans are ignored.
func extractLinks(src string) []link {
	code := codeRanges(src)
	inCode := func(pos int) bool {
		for _, r := range code {
			if pos >= r[0] && pos < r[1] {
				return true
			}
		}
		return false
	}

	var links []link
	for _, m := range reWiki.FindAllStringSubmatchIndex(src, -1) {
		if inCode(m[0]) {
			continue
		}
		l := parseWikilink(src[m[2]:m[3]])
		l.start, l.end = m[0], m[1]
		links = append(links, l)
	}
	for _, m := range reMdLink.FindAllStringSubmatchIndex(src, -1) {
		if inCode(m[0]) {
			continue
		}
		text := src[m[2]:m[3]]
		url := src[m[4]:m[5]]
		if skipURL(url) {
			continue
		}
		target, heading := splitHeading(url)
		links = append(links, link{
			isWiki:  false,
			target:  strings.TrimSpace(target),
			heading: heading,
			alias:   strings.TrimSpace(text),
			start:   m[0],
			end:     m[1],
		})
	}
	sort.SliceStable(links, func(i, j int) bool { return links[i].start < links[j].start })
	for i := range links {
		links[i].display = links[i].displayText()
	}
	return links
}

func parseWikilink(inner string) link {
	l := link{isWiki: true}
	if i := strings.Index(inner, "|"); i >= 0 {
		l.alias = strings.TrimSpace(inner[i+1:])
		inner = inner[:i]
	}
	if i := strings.Index(inner, "#"); i >= 0 {
		l.heading = strings.TrimSpace(inner[i+1:])
		inner = inner[:i]
	}
	l.target = strings.TrimSpace(inner)
	return l
}

// splitHeading removes a trailing #anchor from a markdown link target.
func splitHeading(u string) (target, heading string) {
	if i := strings.Index(u, "#"); i >= 0 {
		return u[:i], u[i+1:]
	}
	return u, ""
}

// skipURL reports whether a markdown link target is external (and thus
// not a note we can open): http(s), mailto, other schemes, or a bare
// in-page #anchor.
func skipURL(u string) bool {
	lu := strings.ToLower(strings.TrimSpace(u))
	switch {
	case lu == "":
		return true
	case strings.HasPrefix(lu, "#"):
		return true
	case strings.HasPrefix(lu, "mailto:"):
		return true
	case strings.Contains(lu, "://"):
		return true
	}
	return false
}

func (l link) displayText() string {
	if l.alias != "" {
		return l.alias
	}
	if l.isWiki && l.heading != "" {
		return l.target + " › " + l.heading
	}
	return l.target
}

// codeRanges returns byte ranges in src that fall inside fenced code
// blocks (``` or ~~~) or inline code spans, so link scanning and the
// pre-render transform can leave them untouched.
func codeRanges(src string) [][2]int {
	var ranges [][2]int
	offset := 0
	inFence := false
	fenceStart := 0
	marker := ""
	for _, line := range strings.SplitAfter(src, "\n") {
		if line == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case !inFence && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")):
			inFence = true
			fenceStart = offset
			marker = trimmed[:3]
		case inFence && strings.HasPrefix(trimmed, marker):
			inFence = false
			ranges = append(ranges, [2]int{fenceStart, offset + len(line)})
		case !inFence:
			for _, m := range reInline.FindAllStringIndex(line, -1) {
				ranges = append(ranges, [2]int{offset + m[0], offset + m[1]})
			}
		}
		offset += len(line)
	}
	if inFence {
		ranges = append(ranges, [2]int{fenceStart, offset})
	}
	return ranges
}

func normStem(s string) string {
	s = path.Base(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, path.Ext(s))
	return strings.ToLower(s)
}

// resolveLinks fills in each link's resolved vault path.
func (m *Model) resolveLinks() {
	note := m.v.Notes[m.path]
	for i := range m.links {
		l := &m.links[i]
		if l.isWiki {
			l.resolved = m.resolveWiki(l.target, note)
		} else {
			l.resolved = m.resolveRelative(l.target)
		}
	}
}

// resolveWiki resolves a wikilink target the way the vault does: prefer
// the note's already-resolved Links, else stem-match the whole index.
func (m *Model) resolveWiki(target string, note *vault.Note) string {
	stem := normStem(target)
	if stem == "" {
		return ""
	}
	if note != nil {
		for _, p := range note.Links {
			if normStem(p) == stem {
				return p
			}
		}
	}
	return m.stemLookup(stem)
}

// resolveRelative resolves a relative markdown link against the current
// note's directory, then falls back to a stem match.
func (m *Model) resolveRelative(target string) string {
	if target == "" {
		return ""
	}
	joined := path.Clean(path.Join(path.Dir(m.path), target))
	joined = strings.TrimPrefix(joined, "./")
	if _, ok := m.v.Notes[joined]; ok {
		return joined
	}
	if !strings.HasSuffix(joined, ".md") {
		if _, ok := m.v.Notes[joined+".md"]; ok {
			return joined + ".md"
		}
	}
	return m.stemLookup(normStem(target))
}

// stemLookup finds the note whose filename stem matches; on ties it
// prefers the shallowest path, then the lexicographically smallest.
func (m *Model) stemLookup(stem string) string {
	var cands []string
	for p := range m.v.Notes {
		if normStem(p) == stem {
			cands = append(cands, p)
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Slice(cands, func(i, j int) bool {
		ci, cj := strings.Count(cands[i], "/"), strings.Count(cands[j], "/")
		if ci != cj {
			return ci < cj
		}
		return cands[i] < cands[j]
	})
	return cands[0]
}
