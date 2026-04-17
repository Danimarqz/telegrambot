package revanced

import (
	"fmt"
	"os"

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
