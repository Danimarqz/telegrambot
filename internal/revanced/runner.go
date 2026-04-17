package revanced

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	resolveTimeout = 5 * time.Minute
	buildTimeout   = 15 * time.Minute
)

// VersionEntry mirrors one element of the resolver's versions.json output.
type VersionEntry struct {
	AppName     string `json:"app_name"`
	PackageName string `json:"package_name"`
	Version     string `json:"suggested_version"`
}

// Resolve runs the revanced resolver (--resolve-only) and returns the list
// of required APKs read from apks/versions.json.
func Resolve(ctx context.Context, repoDir string) ([]VersionEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", filepath.Join(repoDir, "docker-compose-local.yml"),
		"--profile", "build", "run", "--rm", "revanced",
		"python", "main.py", "--resolve-only",
	)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("resolve: %w\n%s", err, string(out))
	}

	versionsPath := filepath.Join(repoDir, "apks", "versions.json")
	data, err := os.ReadFile(versionsPath)
	if err != nil {
		return nil, fmt.Errorf("read versions.json: %w", err)
	}

	var versions []VersionEntry
	if err := json.Unmarshal(data, &versions); err != nil {
		return nil, fmt.Errorf("parse versions.json: %w", err)
	}
	return versions, nil
}

// Build runs the full revanced build pipeline inside docker compose.
// apps is the comma-separated list of app names to patch (e.g. "youtube,youtube_music").
// When empty the container falls back to its .env defaults.
// onLine is called for every stdout/stderr line emitted by the container.
func Build(ctx context.Context, repoDir string, apps string, onLine func(string)) error {
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()

	// Wipe previous outputs so this build's glob is unambiguous.
	apksDir := filepath.Join(repoDir, "apks")
	oldOutputs, _ := filepath.Glob(filepath.Join(apksDir, "Re*-output.apk"))
	for _, p := range oldOutputs {
		_ = os.Remove(p)
	}

	args := []string{
		"compose",
		"-f", filepath.Join(repoDir, "docker-compose-local.yml"),
		"--profile", "build", "run", "--rm",
	}
	// Force unbuffered Python output so we get streaming progress.
	args = append(args, "-e", "PYTHONUNBUFFERED=1")
	if apps != "" {
		args = append(args, "-e", "PATCH_APPS="+apps)
		args = append(args, "-e", "EXISTING_DOWNLOADED_APKS="+apps)
	}
	args = append(args, "revanced")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = repoDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start build: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		if onLine != nil {
			onLine(scanner.Text())
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	return nil
}

// BuiltAPKs returns only the patched output APKs (Re*-output.apk) and
// VancedMicroG from the repo's apks directory, skipping originals.
func BuiltAPKs(repoDir string) ([]string, error) {
	apksDir := filepath.Join(repoDir, "apks")

	patched, err := filepath.Glob(filepath.Join(apksDir, "Re*-output.apk"))
	if err != nil {
		return nil, fmt.Errorf("glob patched: %w", err)
	}

	extras, err := filepath.Glob(filepath.Join(apksDir, "VancedMicroG*.apk"))
	if err != nil {
		return nil, fmt.Errorf("glob microg: %w", err)
	}

	if len(patched) == 0 {
		return nil, fmt.Errorf("build termino sin producir Re*-output.apk — revisa logs del container")
	}

	return append(patched, extras...), nil
}
