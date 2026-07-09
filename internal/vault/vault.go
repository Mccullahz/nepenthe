// Package vault models a knowledge base: a directory tree of markdown
// notes, the links between them, and import/export operations.
package vault

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// scannedFile is an on-disk markdown file discovered by a directory walk,
// before its content has been parsed.
type scannedFile struct {
	path    string // vault-relative, forward-slashed
	abs     string // absolute filesystem path
	modTime time.Time
	size    int64
}

// Open scans root recursively for markdown notes.
func Open(root string) (*Vault, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("vault: open %s: %w", root, err)
	}
	v := &Vault{Root: abs, Notes: map[string]*Note{}}
	if err := v.Rescan(); err != nil {
		return nil, err
	}
	return v, nil
}

// Rescan rebuilds the note index from disk. It reads every markdown file
// exactly once and is safe to call repeatedly (each call rebuilds the
// index from scratch rather than mutating shared state).
func (v *Vault) Rescan() error {
	files, err := scanFiles(v.Root)
	if err != nil {
		return fmt.Errorf("vault: scan %s: %w", v.Root, err)
	}

	// Build lookup structures used to resolve links before parsing note
	// bodies, since any note may link to any other note.
	lowerPaths := make(map[string]string, len(files))
	stemIndex := make(map[string][]string, len(files))
	for _, f := range files {
		lowerPaths[strings.ToLower(f.path)] = f.path
		stem := strings.ToLower(stemOf(f.path))
		stemIndex[stem] = append(stemIndex[stem], f.path)
	}

	notes := make(map[string]*Note, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f.abs)
		if err != nil {
			return fmt.Errorf("vault: read %s: %w", f.path, err)
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")

		fm, body := splitFrontmatter(content)
		title := resolveTitle(fm, body, f.path)
		tags := extractTags(fm.tags, body)
		links := resolveLinks(extractRawTargets(body), f.path, lowerPaths, stemIndex)

		notes[f.path] = &Note{
			Path:    f.path,
			Title:   title,
			Tags:    tags,
			Links:   links,
			ModTime: f.modTime,
			Size:    f.size,
		}
	}

	v.Notes = notes
	return nil
}

// scanFiles walks root and returns every markdown file found, skipping
// hidden directories (dot-prefixed, e.g. ".nepenthe") and hidden files.
func scanFiles(root string) ([]scannedFile, error) {
	var files []scannedFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(p), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, scannedFile{
			path:    filepath.ToSlash(rel),
			abs:     p,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// stemOf returns the filename stem (base name without its final
// extension) of a vault-relative, forward-slashed path.
func stemOf(relPath string) string {
	base := path.Base(relPath)
	return strings.TrimSuffix(base, path.Ext(base))
}
