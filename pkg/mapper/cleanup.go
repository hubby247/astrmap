package mapper

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hubby247/astrmap/pkg/config"
	"github.com/hubby247/astrmap/pkg/fs"
)

func CleanupMaps(cfg config.Config) {
	log.Println("🧹 Cleaning up orphaned maps...")
	cleanedCount := 0

	for _, rc := range cfg.Roots {
		absRoot, _ := filepath.Abs(rc.Path)
		filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// If it's a directory
			if info.IsDir() {
				// Check if ignored
				if fs.ShouldIgnore(path, info, rc.IgnoredDirs) {
					// It is ignored! Remove all map files inside relevant to this folder?
					// Or just remove the folder level maps?
					// If we skip dir, we can't clean inside.
					// We must clean the _level_X.map.txt files IN this dir.

					files := []string{"_level_0.map.txt", "_level_1.map.txt", "_level_2.map.txt", "_level_3.map.txt"}
					for _, f := range files {
						mapPath := filepath.Join(path, f)
						if _, err := os.Stat(mapPath); err == nil {
							os.Remove(mapPath)
							cleanedCount++
						}
					}

					// Also, if we want to be thorough, we should walk inside and delete .map.txt files?
					// But standard Walk will skip if we return SkipDir.
					// We actually want to enter ignored dirs ONLY to clean them.
					return nil // Continue walking to clean children!
				}
			} else {
				// It is a file
				if strings.HasSuffix(path, ".map.txt") {
					// Check if the SOURCE file is ignored or deleted
					// Source file is path without ".map.txt"
					sourcePath := strings.TrimSuffix(path, ".map.txt")

					// If source doesn't exist, delete map
					if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
						os.Remove(path)
						cleanedCount++
						return nil
					}

					// If source exists but is now ignored (by extension or parent folder)
					// Parent folder ignore is handled by recursion above potentially?
					// fs.ShouldIgnore checks path against ignored dirs.
					sourceInfo, err := os.Stat(sourcePath)
					if err == nil {
						if fs.ShouldIgnore(sourcePath, sourceInfo, rc.IgnoredDirs) {
							os.Remove(path)
							cleanedCount++
						} else {
							// Check extension?
							ext := strings.ToLower(filepath.Ext(sourcePath))
							isAllowed := false
							for _, allowed := range rc.AllowedExts {
								if allowed == ext {
									isAllowed = true
									break
								}
							}
							if !isAllowed {
								os.Remove(path)
								cleanedCount++
							}
						}
					}
				}
			}
			return nil
		})
	}
	log.Printf("🧹 Removed %d obsolete map files.", cleanedCount)
}

// DeepClean deletes ALL .map.txt files recursively from the target directory.
func DeepClean(targetDir string) {
	log.Printf("🧹 Performing Deep Clean in: %s", targetDir)
	deleted := 0

	filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".map.txt") {
			os.Remove(path)
			deleted++
		}
		return nil
	})

	log.Printf("✨ Deep Clean finished. Removed %d files.", deleted)
}
