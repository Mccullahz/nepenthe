package noteview

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// Sentinel runes bracket each link's display text in the transformed
// source. They survive glamour rendering as ordinary text, then
// applyHighlights swaps them for theme-styled, selection-aware markup.
// Private-use-area runes are used so they never collide with content.
const (
	sentOpen  = rune(0xE000) // begins "<index>"
	sentMid   = rune(0xE002) // separates index from display text
	sentClose = rune(0xE001) // ends display text
)

var (
	reSentinel = regexp.MustCompile(`(?s)\x{E000}(\d+)\x{E002}(.*?)\x{E001}`)
	reANSI     = regexp.MustCompile("\x1b\\[[0-9;]*m")
)

// transformSource rewrites every extracted link into sentinel-wrapped
// display text so glamour renders readable words instead of raw
// [[brackets]] or [](links). Text inside code (already excluded from
// links) is copied verbatim.
func transformSource(src string, links []link) string {
	if len(links) == 0 {
		return src
	}
	var b strings.Builder
	last := 0
	for i, l := range links {
		if l.start < last {
			continue // overlapping match, skip defensively
		}
		b.WriteString(src[last:l.start])
		b.WriteRune(sentOpen)
		b.WriteString(strconv.Itoa(i))
		b.WriteRune(sentMid)
		b.WriteString(l.display)
		b.WriteRune(sentClose)
		last = l.end
	}
	b.WriteString(src[last:])
	return b.String()
}

// newRenderer builds a glamour renderer honoring the configured style:
// "auto" -> autodetect, a *.json path -> a custom style file, anything
// else -> a named standard style.
func newRenderer(style string, wrap int) (*glamour.TermRenderer, error) {
	opts := []glamour.TermRendererOption{glamour.WithWordWrap(wrap)}
	switch {
	case style == "" || style == "auto":
		opts = append(opts, glamour.WithAutoStyle())
	case strings.HasSuffix(strings.ToLower(style), ".json"):
		opts = append(opts, glamour.WithStylesFromJSONFile(style))
	default:
		opts = append(opts, glamour.WithStandardStyle(style))
	}
	return glamour.NewTermRenderer(opts...)
}

// applyHighlights replaces the sentinel markers left in the rendered
// output with styled link text: the current link stands out strongly,
// the rest are underlined subtly.
func (m *Model) applyHighlights(rendered string) string {
	return reSentinel.ReplaceAllStringFunc(rendered, func(match string) string {
		sm := reSentinel.FindStringSubmatch(match)
		idx, _ := strconv.Atoi(sm[1])
		text := reANSI.ReplaceAllString(sm[2], "")
		if idx == m.current {
			return m.curLinkStyle.Render(text)
		}
		return m.linkStyle.Render(text)
	})
}

func (m *Model) buildStyles() {
	accent := lipgloss.Color(m.cfg.Theme.Accent)
	m.linkStyle = lipgloss.NewStyle().Foreground(accent).Underline(true)
	m.curLinkStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("231")).
		Background(accent).
		Bold(true)
}
