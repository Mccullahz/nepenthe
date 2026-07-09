package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read returns a note's raw markdown.
func (v *Vault) Read(path string) (string, error) {
	full := filepath.Join(v.Root, filepath.FromSlash(path))
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("vault: read %s: %w", path, err)
	}
	return string(data), nil
}

// Write saves raw markdown to a note, creating parent directories. It
// does not update the in-memory index; call Rescan afterwards if the
// index needs to reflect the change.
func (v *Vault) Write(path, content string) error {
	full := filepath.Join(v.Root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("vault: write %s: %w", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return fmt.Errorf("vault: write %s: %w", path, err)
	}
	return nil
}

// Create makes a new empty note; errors if it already exists.
func (v *Vault) Create(path string) error {
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		path += ".md"
	}
	if _, ok := v.Notes[path]; ok {
		return fmt.Errorf("vault: create %s: note already exists", path)
	}
	full := filepath.Join(v.Root, filepath.FromSlash(path))
	if _, err := os.Stat(full); err == nil {
		return fmt.Errorf("vault: create %s: note already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("vault: create %s: %w", path, err)
	}

	title := stemOf(path)
	if err := v.Write(path, "# "+title+"\n\n"); err != nil {
		return err
	}
	return v.Rescan()
}

// Delete removes a note from disk and the index.
func (v *Vault) Delete(path string) error {
	full := filepath.Join(v.Root, filepath.FromSlash(path))
	if err := os.Remove(full); err != nil {
		return fmt.Errorf("vault: delete %s: %w", path, err)
	}
	return v.Rescan()
}
