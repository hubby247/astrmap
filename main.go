package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hubby247/astrmap/pkg/config"
	"github.com/hubby247/astrmap/pkg/fs"
	"github.com/hubby247/astrmap/pkg/mapper"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "scan":
		targetDir := "."
		if len(os.Args) >= 3 {
			targetDir = os.Args[2]
		}
		runScan(targetDir)
	case "clean":
		targetDir := "."
		if len(os.Args) >= 3 {
			targetDir = os.Args[2]
		}
		runClean(targetDir)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("🗺️  AstrMap - The AST Indexer for LLMs")
	fmt.Println("\nUsage:")
	fmt.Println("  astrmap scan [directory]   Scan directory and generate .map.txt files")
	fmt.Println("  astrmap clean              Remove all .map.txt files in the current workspace")
}

func runScan(targetDir string) {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	fmt.Printf("🚀 Scanning %s...\n", absTarget)
	start := time.Now()

	// Load or mock config
	cfg := config.LoadOrSetup()

	// 1. Find all allowed files
	var allFiles []string
	validExts := make(map[string]bool)
	for _, root := range cfg.Roots {
		for _, ext := range root.AllowedExts {
			validExts[ext] = true
		}
	}

	filepath.Walk(absTarget, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Check against git / node_modules default ignores if not in config
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			for _, root := range cfg.Roots {
				if fs.ShouldIgnore(path, info, root.IgnoredDirs) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := filepath.Ext(path)
		if validExts[ext] {
			allFiles = append(allFiles, path)
		}
		return nil
	})

	fmt.Printf("Found %d files to map.\n", len(allFiles))

	// 2. Map individual files
	for _, f := range allFiles {
		mapper.GenerateMap(f)
	}

	// 3. Generate Level Maps
	mapper.GenerateFolderMaps(cfg, allFiles)

	fmt.Printf("✅ Mapping complete in %v. Check the _level_*.map.txt files!\n", time.Since(start))
}

func runClean(targetDir string) {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}
	mapper.DeepClean(absTarget)
	fmt.Println("✅ Workspace cleaned.")
}
