package fs

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var (
	IgnoredDirs = []string{".git", "node_modules", "dist", "bin", "vendor", ".idea", ".vscode"}
)

func ShouldIgnore(path string, info os.FileInfo, extraIgnores []string) bool {
	return ShouldIgnoreName(info.Name(), extraIgnores)
}

func ShouldIgnoreName(name string, extra []string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	for _, ignored := range IgnoredDirs {
		if strings.EqualFold(name, ignored) {
			return true
		}
	}
	for _, ignored := range extra {
		if strings.EqualFold(name, ignored) {
			return true
		}
	}
	return false
}

func CountLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	count := 0
	for s.Scan() {
		count++
	}
	return count
}

func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
