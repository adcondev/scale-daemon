package logging

import (
	"fmt"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"os"
	"path/filepath"
	"strings"
)

// secureFilepath constructs an absolute file path from baseDir and relPath,
// ensuring that the resulting path is within baseDir to prevent directory traversal.
func secureFilepath(baseDir, relPath string) (string, error) {
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("invalid base directory: %w", err)
	}

	target := filepath.Join(baseAbs, relPath)
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("invalid target path: %w", err)
	}

	// Resolve symlinks to prevent escaping
	if eval, err := filepath.EvalSymlinks(baseAbs); err == nil {
		baseAbs = eval
	}
	if eval, err := filepath.EvalSymlinks(targetAbs); err == nil {
		targetAbs = eval
	}

	// Ensure target is inside base directory
	if targetAbs != baseAbs && !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("file %s is outside allowed directory", relPath)
	}

	return targetAbs, nil
}
