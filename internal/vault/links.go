package vault

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	// wikilinkCaptureRe captures the target of [[target]], [[target|alias]],
	// [[target#heading]] and [[target#heading|alias]] forms.
	wikilinkCaptureRe = regexp.MustCompile(`\[\[([^\]|#]+)(?:#[^\]|]*)?(?:\|[^\]]*)?\]\]`)
	// mdLinkCaptureRe captures the target of [text](target) forms.
	mdLinkCaptureRe = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	// schemeRe recognizes an absolute URL with a scheme, e.g. "https://".
	schemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
)

// extractRawTargets pulls raw (unresolved) link targets out of a note
// body: wikilink targets and markdown-link targets, excluding external
// links (http/https/mailto/absolute URLs).
func extractRawTargets(body string) []string {
	var out []string

	for _, m := range wikilinkCaptureRe.FindAllStringSubmatch(body, -1) {
		if t := strings.TrimSpace(m[1]); t != "" {
			out = append(out, t)
		}
	}

	for _, m := range mdLinkCaptureRe.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(m[2])
		if target == "" || isExternalLink(target) {
			continue
		}
		// A "(url "title")" form: drop a trailing quoted title.
		if idx := strings.IndexAny(target, " \t"); idx >= 0 {
			target = target[:idx]
		}
		if idx := strings.IndexByte(target, '#'); idx >= 0 {
			target = target[:idx]
		}
		if unescaped, err := url.PathUnescape(target); err == nil {
			target = unescaped
		}
		target = strings.TrimSpace(target)
		if target != "" {
			out = append(out, target)
		}
	}

	return out
}

func isExternalLink(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "mailto:") {
		return true
	}
	if strings.HasPrefix(target, "/") {
		return true // filesystem-absolute
	}
	return schemeRe.MatchString(target)
}

// resolveLinks resolves a note's raw link targets to vault-relative note
// paths, dropping unresolved links, self-links, and duplicates.
func resolveLinks(rawTargets []string, fromPath string, lowerPaths map[string]string, stemIndex map[string][]string) []string {
	if len(rawTargets) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(rawTargets))
	var out []string
	for _, raw := range rawTargets {
		resolved, ok := resolveTarget(raw, lowerPaths, stemIndex)
		if !ok || resolved == fromPath || seen[resolved] {
			continue
		}
		seen[resolved] = true
		out = append(out, resolved)
	}
	return out
}

// resolveTarget implements Obsidian-like, case-insensitive link
// resolution:
//   - a target containing "/" is resolved as a vault-relative path
//     (".md" appended if missing);
//   - otherwise the target is matched by filename stem anywhere in the
//     vault; if several notes share a stem, the shortest path wins, ties
//     broken lexicographically.
func resolveTarget(target string, lowerPaths map[string]string, stemIndex map[string][]string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", false
	}
	target = strings.ReplaceAll(target, "\\", "/")

	if strings.Contains(target, "/") {
		p := strings.TrimPrefix(target, "./")
		if !strings.HasSuffix(strings.ToLower(p), ".md") {
			p += ".md"
		}
		cleaned := path.Clean(p)
		if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "..") {
			return "", false
		}
		if actual, ok := lowerPaths[strings.ToLower(cleaned)]; ok {
			return actual, true
		}
		return "", false
	}

	stem := target
	if strings.HasSuffix(strings.ToLower(stem), ".md") {
		stem = stem[:len(stem)-len(".md")]
	}
	candidates := stemIndex[strings.ToLower(stem)]
	switch len(candidates) {
	case 0:
		return "", false
	case 1:
		return candidates[0], true
	default:
		sorted := append([]string(nil), candidates...)
		sort.Slice(sorted, func(i, j int) bool {
			if len(sorted[i]) != len(sorted[j]) {
				return len(sorted[i]) < len(sorted[j])
			}
			return sorted[i] < sorted[j]
		})
		return sorted[0], true
	}
}
