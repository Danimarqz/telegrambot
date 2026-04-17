package revanced

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Publish copies all built APKs from the repo build directory to serveDir
// and returns the list of file names that were copied.
func Publish(repoDir, serveDir string) ([]string, error) {
	apks, err := BuiltAPKs(repoDir)
	if err != nil {
		return nil, err
	}
	if len(apks) == 0 {
		return nil, fmt.Errorf("no APKs found in build directory")
	}

	if err := os.MkdirAll(serveDir, 0755); err != nil {
		return nil, fmt.Errorf("create serve dir: %w", err)
	}

	// Remove previous build outputs from serve dir before copying.
	oldPublished, _ := filepath.Glob(filepath.Join(serveDir, "Re*-output.apk"))
	for _, p := range oldPublished {
		_ = os.Remove(p)
	}

	var published []string
	for _, src := range apks {
		name := filepath.Base(src)
		dst := filepath.Join(serveDir, name)
		if err := copyFile(src, dst); err != nil {
			return published, fmt.Errorf("copy %s: %w", name, err)
		}
		published = append(published, name)
	}
	return published, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
