package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// MaxLogSize is the threshold for log rotation (5MB)
const MaxLogSize = 5 * 1024 * 1024

// RotateIfNeeded truncates the log file if it exceeds MaxLogSize
// Keeps the last 1000 lines for continuity
func RotateIfNeeded(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Size() < MaxLogSize {
		return nil
	}

	lines := ReadLastNLines(path, 1000)
	if len(lines) == 0 {
		return nil
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0600)
}

// ReadLastNLines reads the last n lines from a file efficiently
func ReadLastNLines(path string, n int) []string {
	baseDir := filepath.Dir(path)
	relPath := filepath.Base(path)
	securePath, err := secureFilepath(baseDir, relPath)
	if err != nil {
		return []string{}
	}
	file, err := os.Open(securePath) //nolint:gosec
	if err != nil {
		return []string{}
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("[!] Error al cerrar archivo: %v", err)
		}
	}(file)

	stat, err := file.Stat()
	if err != nil {
		return []string{}
	}

	size := stat.Size()
	if size == 0 {
		return []string{}
	}

	// Read last 64KB max (sufficient for ~1000 lines)
	bufSize := int64(64 * 1024)
	if size < bufSize {
		bufSize = size
	}

	buf := make([]byte, bufSize)
	_, err = file.Seek(size-bufSize, io.SeekStart)
	if err != nil {
		return []string{}
	}

	_, err = file.Read(buf)
	if err != nil {
		return []string{}
	}

	allLines := strings.Split(string(buf), "\n")

	// Clean empty lines at end
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	// If we started mid-line, discard first partial line
	if size > bufSize && len(allLines) > 0 {
		allLines = allLines[1:]
	}

	if len(allLines) <= n {
		return allLines
	}
	return allLines[len(allLines)-n:]
}

// Flush reduces the log file to the last 50 lines
func Flush(path string) error {
	lines := ReadLastNLines(path, 50)
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(content), 0600)
}

// GetFileSize returns the size of the log file in bytes
func GetFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
