package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

const (
	// maxFingerprintFiles and maxFingerprintBytes bound duplicate comparison
	// work. A skill exceeding either bound is not fingerprinted; any group
	// holding it is treated as conflicting so callers ask instead of guessing.
	maxFingerprintFiles = 512
	maxFingerprintBytes = 8 << 20
)

// skillFingerprint is the content hash of one skill directory. An empty hash
// means the directory could not be compared (too large) and must never match
// another copy.
type skillFingerprint struct {
	hash string
}

// short returns the display prefix of a comparable hash and "-" otherwise.
func (f skillFingerprint) short() string {
	if len(f.hash) < 12 {
		return "-"
	}
	return f.hash[:12]
}

// fingerprintSkillDir hashes every file and symlink inside a skill directory.
// Entries are hashed in relative-path order over names, entry kinds, and file
// bytes, so identical copies hash equally regardless of absolute location.
// Directory traversal never follows symlinks.
func fingerprintSkillDir(dir string) (skillFingerprint, error) {
	type entry struct {
		relative string
		path     string
		info     fs.FileInfo
	}
	var entries []entry
	err := filepath.WalkDir(dir, func(currentPath string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == dir {
			return nil
		}
		if dirEntry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(dir, currentPath)
		if err != nil {
			return fmt.Errorf("resolve relative skill entry for %s: %w", currentPath, err)
		}
		info, err := dirEntry.Info()
		if err != nil {
			return fmt.Errorf("inspect skill entry %s: %w", currentPath, err)
		}
		entries = append(entries, entry{relative: filepath.ToSlash(relative), path: currentPath, info: info})
		return nil
	})
	if err != nil {
		return skillFingerprint{}, fmt.Errorf("list skill directory %s: %w", dir, err)
	}
	if len(entries) > maxFingerprintFiles {
		return skillFingerprint{}, nil
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].relative < entries[j].relative
	})

	hasher := sha256.New()
	var hashedBytes int64
	for _, item := range entries {
		mode := item.info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(item.path)
			if err != nil {
				return skillFingerprint{}, fmt.Errorf("read skill symlink %s: %w", item.path, err)
			}
			fmt.Fprintf(hasher, "link\x00%s\x00%s\x00", item.relative, target)
		case mode.IsRegular():
			size := item.info.Size()
			if hashedBytes+size > maxFingerprintBytes {
				return skillFingerprint{}, nil
			}
			file, err := os.Open(item.path)
			if err != nil {
				return skillFingerprint{}, fmt.Errorf("read skill file %s: %w", item.path, err)
			}
			fmt.Fprintf(hasher, "file\x00%s\x00%s\x00", item.relative, strconv.FormatInt(size, 10))
			written, copyErr := io.Copy(hasher, file)
			closeErr := file.Close()
			if copyErr != nil {
				return skillFingerprint{}, fmt.Errorf("read skill file %s: %w", item.path, copyErr)
			}
			if closeErr != nil {
				return skillFingerprint{}, fmt.Errorf("read skill file %s: %w", item.path, closeErr)
			}
			hashedBytes += written
			hasher.Write([]byte{0})
		default:
			fmt.Fprintf(hasher, "other\x00%s\x00%s\x00", item.relative, mode.Type().String())
		}
	}
	return skillFingerprint{hash: hex.EncodeToString(hasher.Sum(nil))}, nil
}
