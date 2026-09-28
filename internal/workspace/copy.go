package workspace

import (
	"os"
)

// CopyFile writes the bytes of src to dst atomically (temp file in the same
// directory, then rename) and returns the SHA-256 of the content written.
// The executable bits of src are preserved so skill scripts stay runnable.
// An existing symlink at dst is replaced by the rename, never followed.
func CopyFile(src string, dst string) (string, error) {
	content, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	perm := info.Mode().Perm() | 0o600

	tmp := dst + ".csaw-tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return ContentSHA256(content), nil
}
