package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/hubby247/astrmap/pkg/fs"
)

const (
	ConfigFile = "codemap.json"
)

// RootConfig defines a directory to map
type RootConfig struct {
	Path        string   `json:"path"`
	AllowedExts []string `json:"allowed_exts"`
	IgnoredDirs []string `json:"ignored_dirs,omitempty"`
}

// Config holds the application configuration
type Config struct {
	Roots []RootConfig `json:"roots"`
	// Transient Command Field (for IPC via file)
	Command *CommandPayload `json:"_command,omitempty"`
}

type CommandPayload struct {
	Action    string `json:"action"`
	Mode      string `json:"mode"`
	Ext       string `json:"ext"`
	Timestamp int64  `json:"timestamp"`
}

// LoadOrSetup tries to load config or starts interactive setup
func LoadOrSetup() Config {
	// Try load
	if _, err := os.Stat(ConfigFile); err == nil {
		data, err := os.ReadFile(ConfigFile)
		if err == nil {
			var cfg Config
			// Try V2 format
			if err := json.Unmarshal(data, &cfg); err == nil {
				if len(cfg.Roots) > 0 {
					log.Printf("✅ Loaded config with %d roots.", len(cfg.Roots))
					return cfg
				}
			}

			// Fallback: Try V1 format and migrate
			var v1 struct {
				AllowedExts []string `json:"allowed_exts"`
				RootPath    string   `json:"root_path"`
			}
			if err := json.Unmarshal(data, &v1); err == nil {
				log.Println("⚠️ Detected V1 config. Migrating to V2...")
				if v1.RootPath == "" || v1.RootPath == "." {
					abs, _ := filepath.Abs(".")
					v1.RootPath = abs
				}
				newCfg := Config{
					Roots: []RootConfig{
						{Path: v1.RootPath, AllowedExts: v1.AllowedExts},
					},
				}
				Save(newCfg)
				return newCfg
			}
		}
	}

	// Interactive Setup
	log.Println("🔍 No valid config found. Starting interactive setup...")
	return initInteractiveSetup()
}

func Save(cfg Config) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(ConfigFile, data, 0644)
}

func initInteractiveSetup() Config {
	// Default Root is CWD
	cwd, _ := os.Getwd()
	absRoot, _ := filepath.Abs(cwd)

	log.Printf("🔍 Scanning directory '%s' for file types...", absRoot)

	extCounts := make(map[string]int)
	filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || fs.ShouldIgnore(path, info, nil) {
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext != "" {
				extCounts[ext]++
			}
		}
		return nil
	})

	var suggestedExts []string
	for ext, count := range extCounts {
		if count > 0 {
			suggestedExts = append(suggestedExts, ext)
		}
	}

	log.Println("Found extensions:", suggestedExts)
	log.Println("Auto-configuring to watch these extensions.")

	cfg := Config{
		Roots: []RootConfig{
			{
				Path:        absRoot,
				AllowedExts: suggestedExts,
				IgnoredDirs: []string{"node_modules", ".git", "dist", "build"},
			},
		},
	}
	Save(cfg)
	return cfg
}
