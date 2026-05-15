package revanced

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shogo82148/androidbinary/apk"
)

// APKInfo holds the package name and version extracted from an APK file.
type APKInfo struct {
	PackageName string
	VersionName string
}

// ReadAPKInfo opens the file at path and extracts package name + version.
func ReadAPKInfo(path string) (APKInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return APKInfo{}, fmt.Errorf("open apk: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return APKInfo{}, fmt.Errorf("stat apk: %w", err)
	}

	pkg, err := apk.OpenZipReader(f, fi.Size())
	if err != nil {
		return APKInfo{}, fmt.Errorf("parse apk: %w", err)
	}
	defer pkg.Close()

	name := pkg.PackageName()
	version, _ := pkg.Manifest().VersionName.String()
	if name == "" {
		return APKInfo{}, fmt.Errorf("apk has no package name")
	}

	return APKInfo{
		PackageName: name,
		VersionName: version,
	}, nil
}

// ReadAPKMInfo extracts base.apk from an APKMirror bundle (.apkm) and
// returns its package + version. The .apkm is a ZIP containing base.apk
// alongside split_config.*.apk files.
func ReadAPKMInfo(path string) (APKInfo, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return APKInfo{}, fmt.Errorf("open apkm: %w", err)
	}
	defer zr.Close()

	var baseEntry *zip.File
	for _, f := range zr.File {
		if f.Name == "base.apk" {
			baseEntry = f
			break
		}
	}
	if baseEntry == nil {
		return APKInfo{}, fmt.Errorf("apkm has no base.apk")
	}

	rc, err := baseEntry.Open()
	if err != nil {
		return APKInfo{}, fmt.Errorf("open base.apk inside apkm: %w", err)
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "apkm-base-*.apk")
	if err != nil {
		return APKInfo{}, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, rc); err != nil {
		tmp.Close()
		return APKInfo{}, fmt.Errorf("extract base.apk: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return APKInfo{}, fmt.Errorf("close temp: %w", err)
	}

	return ReadAPKInfo(tmpPath)
}

// ReadBundleInfo dispatches to ReadAPKInfo or ReadAPKMInfo based on the
// file extension. Use this when the caller accepts either format.
func ReadBundleInfo(path string) (APKInfo, error) {
	if strings.HasSuffix(strings.ToLower(path), ".apkm") {
		return ReadAPKMInfo(path)
	}
	return ReadAPKInfo(path)
}
