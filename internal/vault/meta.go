package vault

import (
	"regexp"
	"strings"
)

// frontmatter holds the subset of YAML frontmatter fields nepenthe cares
// about. Parsing is hand-rolled (no YAML dependency): it only understands
// a leading `---`-delimited block with simple `key: value` lines, inline
// `tags: [a, b]` lists, and block-style `tags:` followed by `- item`
// lines.
type frontmatter struct {
	title    string
	hasTitle bool
	tags     []string
}

var fmKeyRe = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*(.*)$`)

// splitFrontmatter separates a leading frontmatter block (if any) from
// the note body. If there is no well-formed `---`...`---` block at the
// very start of the content, the whole content is returned as the body.
func splitFrontmatter(content string) (frontmatter, string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, content
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return frontmatter{}, content
	}

	var fm frontmatter
	fmLines := lines[1:end]
	for i := 0; i < len(fmLines); i++ {
		trimmed := strings.TrimSpace(fmLines[i])
		if trimmed == "" {
			continue
		}
		m := fmKeyRe.FindStringSubmatch(fmLines[i])
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		val := strings.TrimSpace(m[2])

		switch key {
		case "title":
			if val != "" {
				fm.title = unquote(val)
				fm.hasTitle = true
			}
		case "tags":
			if val != "" {
				fm.tags = append(fm.tags, parseInlineTagList(val)...)
				continue
			}
			// Block style: subsequent indented "- item" lines.
			for i+1 < len(fmLines) {
				next := strings.TrimSpace(fmLines[i+1])
				if next == "" {
					i++
					continue
				}
				if !strings.HasPrefix(next, "-") {
					break
				}
				item := unquote(strings.TrimSpace(strings.TrimPrefix(next, "-")))
				if item != "" {
					fm.tags = append(fm.tags, item)
				}
				i++
			}
		}
	}

	body := ""
	if end+1 < len(lines) {
		body = strings.Join(lines[end+1:], "\n")
	}
	return fm, body
}

// parseInlineTagList parses a frontmatter tags value that appeared on the
// `tags:` line itself: either a bracketed list `[a, b]`, a comma
// separated scalar list `a, b`, or a single bare tag.
func parseInlineTagList(val string) []string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		val = val[1 : len(val)-1]
	}
	if !strings.Contains(val, ",") {
		if t := unquote(strings.TrimSpace(val)); t != "" {
			return []string{t}
		}
		return nil
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := unquote(strings.TrimSpace(p)); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// unquote strips a single layer of matching single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

var h1Re = regexp.MustCompile(`^#[ \t]+(.+)$`)

// firstH1 returns the text of the first level-1 ATX heading in body,
// skipping fenced code blocks, or "" if there is none.
func firstH1(body string) string {
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(t, "##") {
			continue // level 2+ heading, not H1
		}
		m := h1Re.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		text := strings.TrimSpace(m[1])
		text = strings.TrimRight(text, "# \t") // strip optional closing ATX hashes
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
	}
	return ""
}

// resolveTitle applies the documented title precedence: frontmatter
// title, then first H1, then filename stem.
func resolveTitle(fm frontmatter, body, relPath string) string {
	if fm.hasTitle && strings.TrimSpace(fm.title) != "" {
		return fm.title
	}
	if h := firstH1(body); h != "" {
		return h
	}
	return stemOf(relPath)
}

// tagRe matches inline #tag tokens: a '#' preceded by whitespace/start of
// line or an opening bracket, immediately followed by a word character
// (so it can't be mistaken for an ATX heading marker, which requires a
// space after the '#').
var tagRe = regexp.MustCompile(`(?m)(?:^|[\s([{])#([A-Za-z0-9_][\w/-]*)`)

var (
	fencedCodeRe   = regexp.MustCompile(`(?s)` + "```" + `.*?` + "```")
	inlineCodeRe   = regexp.MustCompile("`[^`\n]*`")
	wikilinkSpanRe = regexp.MustCompile(`\[\[[^\]]*\]\]`)
	mdLinkSpanRe   = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

// maskForTags blanks out regions of body that should not be scanned for
// inline tags: fenced/inline code and link targets (wikilinks and
// markdown links use '#' for heading anchors, which would otherwise be
// misread as tags). Blanking preserves newlines and byte offsets so the
// rest of the string layout is unaffected.
func maskForTags(body string) string {
	b := []byte(body)
	for _, re := range []*regexp.Regexp{fencedCodeRe, wikilinkSpanRe, mdLinkSpanRe, inlineCodeRe} {
		for _, span := range re.FindAllIndex(b, -1) {
			for i := span[0]; i < span[1]; i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
		}
	}
	return string(b)
}

// extractTags returns frontmatter tags plus inline #tags found in the
// body, deduplicated (order-preserving, first occurrence wins).
func extractTags(fmTags []string, body string) []string {
	combined := make([]string, 0, len(fmTags))
	combined = append(combined, fmTags...)

	masked := maskForTags(body)
	for _, m := range tagRe.FindAllStringSubmatch(masked, -1) {
		combined = append(combined, m[1])
	}

	seen := make(map[string]bool, len(combined))
	out := make([]string, 0, len(combined))
	for _, t := range combined {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
