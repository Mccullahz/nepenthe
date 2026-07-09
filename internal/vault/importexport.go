package vault

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// assetExts are non-markdown file types worth carrying along when
// importing a directory tree, since notes commonly reference them.
var assetExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".pdf":  true,
}

// Import copies an external markdown file or directory tree into the
// vault under destDir ("" = root) and reindexes.
//
// For a single file, the destination must not already exist. For a
// directory, markdown and common asset files are copied recursively,
// preserving relative structure; individual name collisions are skipped
// (not overwritten) and reported in the returned error, but do not abort
// the rest of the import.
func (v *Vault) Import(src, destDir string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("vault: import %s: %w", src, err)
	}
	destBase := filepath.Join(v.Root, filepath.FromSlash(destDir))

	if !info.IsDir() {
		if !strings.EqualFold(filepath.Ext(src), ".md") {
			return fmt.Errorf("vault: import %s: not a markdown file", src)
		}
		dest := filepath.Join(destBase, filepath.Base(src))
		if err := copyFileNoOverwrite(src, dest); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("vault: import %s: destination already exists: %s", src, dest)
			}
			return fmt.Errorf("vault: import %s: %w", src, err)
		}
		return v.Rescan()
	}

	var skipped []string
	walkErr := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != src && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := filepath.Ext(p)
		if !strings.EqualFold(ext, ".md") && !assetExts[strings.ToLower(ext)] {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		dest := filepath.Join(destBase, rel)
		if err := copyFileNoOverwrite(p, dest); err != nil {
			if os.IsExist(err) {
				skipped = append(skipped, filepath.ToSlash(rel))
				return nil
			}
			return err
		}
		return nil
	})

	if rerr := v.Rescan(); rerr != nil {
		if walkErr != nil {
			return fmt.Errorf("vault: import %s: %w (rescan also failed: %v)", src, walkErr, rerr)
		}
		return fmt.Errorf("vault: import %s: rescan failed: %w", src, rerr)
	}
	if walkErr != nil {
		return fmt.Errorf("vault: import %s: %w", src, walkErr)
	}
	if len(skipped) > 0 {
		sort.Strings(skipped)
		return fmt.Errorf("vault: import %s: skipped %d existing file(s): %s", src, len(skipped), strings.Join(skipped, ", "))
	}
	return nil
}

// Export copies a note (path != "") or, with path == "", the whole
// vault's markdown tree out to dest. A leading "~/" (or exactly "~") in
// dest is expanded to the user's home directory.
//
// For a single note, if dest is an existing directory the note is placed
// inside it under its own base name; otherwise dest is treated as the
// literal target file path (parent directories are created). For a whole
// vault export, dest is created as a directory if missing, structure is
// preserved, and existing files are never overwritten: any conflicts are
// skipped and reported in the returned error, but the rest still export.
func (v *Vault) Export(path, dest string) error {
	dest = expandHome(dest)

	if path != "" {
		srcAbs := filepath.Join(v.Root, filepath.FromSlash(path))
		if _, err := os.Stat(srcAbs); err != nil {
			return fmt.Errorf("vault: export %s: %w", path, err)
		}

		target := dest
		if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
			target = filepath.Join(dest, filepath.Base(path))
		} else if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("vault: export %s: %w", path, err)
		}

		if err := copyFileNoOverwrite(srcAbs, target); err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("vault: export %s: destination already exists: %s", path, target)
			}
			return fmt.Errorf("vault: export %s: %w", path, err)
		}
		return nil
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("vault: export: %w", err)
	}

	paths := make([]string, 0, len(v.Notes))
	for p := range v.Notes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var conflicts []string
	for _, p := range paths {
		srcAbs := filepath.Join(v.Root, filepath.FromSlash(p))
		target := filepath.Join(dest, filepath.FromSlash(p))
		if err := copyFileNoOverwrite(srcAbs, target); err != nil {
			if os.IsExist(err) {
				conflicts = append(conflicts, p)
				continue
			}
			return fmt.Errorf("vault: export: %w", err)
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("vault: export: skipped %d existing file(s): %s", len(conflicts), strings.Join(conflicts, ", "))
	}
	return nil
}

// copyFileNoOverwrite copies srcAbs to destAbs, creating parent
// directories as needed. It refuses to overwrite an existing destination
// file, returning an error satisfying os.IsExist in that case.
func copyFileNoOverwrite(srcAbs, destAbs string) error {
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return err
	}
	in, err := os.Open(srcAbs)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// expandHome expands a leading "~" or "~/" in dest to the user's home
// directory. Any other value is returned unchanged.
func expandHome(dest string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return dest
	}
	if dest == "~" {
		return home
	}
	if strings.HasPrefix(dest, "~/") {
		return filepath.Join(home, dest[2:])
	}
	return dest
}
