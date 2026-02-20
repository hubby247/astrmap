package mapper

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/hubby247/astrmap/pkg/config"
	"github.com/hubby247/astrmap/pkg/fs"
)

type Region struct {
	Start int
	End   int
	Name  string
}

// GenerateMap scans a single file and creates a .map.txt file
func GenerateMap(path string) {
	if strings.HasSuffix(path, ".map.txt") || strings.HasSuffix(path, "codemap.json") || strings.HasSuffix(path, "watchlist.txt") {
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("❌ Failed to open source: %s (%v)", filepath.Base(path), err)
		return
	}
	defer file.Close()

	// Metadata
	info, err := file.Stat()
	if err != nil {
		return
	}
	modTime := info.ModTime().Format("2006-01-02 15:04:05")
	sizeKB := float64(info.Size()) / 1024.0

	ext := strings.ToLower(filepath.Ext(path))
	var regions []Region
	var scanner = bufio.NewScanner(file)

	// --- SMART PARSING STATE ---
	type Scope struct {
		Region    Region
		OpenLevel int    // The brace/paren level when this region started
		Indent    int    // Indentation level (for Python)
		CloseChar string // "}" or ")" or "</tag>" (for python or html)
		Tag       string // For HTML matching
	}
	var scopeStack []Scope

	// Counting State
	braceLevel := 0
	parenLevel := 0

	// Language Config
	isBraceLang := (ext == ".go" || ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" || ext == ".java" || ext == ".cs" || ext == ".css" || ext == ".scss" || ext == ".less")
	isPython := (ext == ".py")
	isHtml := (ext == ".html" || ext == ".htm" || ext == ".xml" || ext == ".vue" || ext == ".php")

	lineNum := 0

	// Regex Patterns
	codeRe := regexp.MustCompile(`^\s*//\s*(?:(\d+)\.|#region|={3})\s*(.*)$`)
	mdRe := regexp.MustCompile(`^(#+)\s+(.*)$`)

	// HTML: Matches <tag ... > or </tag>
	htmlTagRe := regexp.MustCompile(`<\s*([a-zA-Z0-9-]+)\b([^>]*)>|<\s*/\s*([a-zA-Z0-9-]+)\s*>`)

	// GO
	goFuncRe := regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?([A-Z][a-zA-Z0-9_]*)\s*\(`)
	goTypeRe := regexp.MustCompile(`^type\s+([A-Z][a-zA-Z0-9_]*)\s+(struct|interface)`)
	goImportRe := regexp.MustCompile(`^import\s*\(`)
	goConstRe := regexp.MustCompile(`^const\s*\(`)
	goVarRe := regexp.MustCompile(`^var\s*\(`)

	// JS/TS
	jsFuncRe := regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+([a-zA-Z0-9_]+)\s*\(`)
	jsClassRe := regexp.MustCompile(`^(?:export\s+)?class\s+([a-zA-Z0-9_]+)`)
	jsArrowRe := regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([a-zA-Z0-9_]+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[a-zA-Z0-9_]+)\s*=>`)
	tsInterfaceRe := regexp.MustCompile(`^(?:export\s+)?interface\s+([a-zA-Z0-9_]+)`)

	// JS Testing & Objects
	jsDescribeRe := regexp.MustCompile(`^(?:describe|context|suite)\s*\(\s*["']([^"']+)["']`)
	jsItRe := regexp.MustCompile(`^(?:it|test)\s*\(\s*["']([^"']+)["']`)
	jsObjRe := regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([a-zA-Z0-9_]+)\s*=\s*\{`)
	jsExportDefaultRe := regexp.MustCompile(`^export\s+default\s*\{`)
	jsRouteRe := regexp.MustCompile(`^(?:router|app)\.(get|post|put|delete|patch|use)\s*\(\s*["']([^"']+)["']`)

	// JS Granular: Methods, Properties
	// RE2 Compatible: No negative lookahead. match broadly, filter later.
	jsMethodRe := regexp.MustCompile(`^\s*(?:async\s+)?([a-zA-Z0-9_]+)\s*\([^)]*\)\s*\{`)

	jsPropFuncRe := regexp.MustCompile(`^\s*([a-zA-Z0-9_]+)\s*:\s*(?:async\s+)?function\s*\(`)
	jsPropArrowRe := regexp.MustCompile(`^\s*([a-zA-Z0-9_]+)\s*:\s*(?:async\s+)?(?:\([^)]*\)|[a-zA-Z0-9_]+)\s*=>`)

	// Ignore keywords for jsMethodRe
	jsKeywords := map[string]bool{
		"if": true, "for": true, "while": true, "switch": true,
		"catch": true, "function": true, "return": true, "await": true, "else": true,
	}

	// CSS/SCSS
	cssRe := regexp.MustCompile(`^([^{]+)\{\s*$`)

	// Dependencies Extractors (Imports)
	depGoRe := regexp.MustCompile(`^\s*import\s*(?:\(\s*)?["']([^"']+)["']`)
	depJsRe := regexp.MustCompile(`^\s*(?:import\s+.*from\s+|require\s*\(\s*)["']([^"']+)["']`)
	depHtmlScriptRe := regexp.MustCompile(`<\s*script\s+[^>]*src=["']([^"']+)["']`)
	depHtmlLinkRe := regexp.MustCompile(`<\s*link\s+[^>]*href=["']([^"']+)["'][^>]*rel=["']stylesheet["']|<\s*link\s+[^>]*rel=["']stylesheet["'][^>]*href=["']([^"']+)["']`)
	depCssRe := regexp.MustCompile(`@import\s*(?:url\s*\(\s*)?["']?([^"'\)]+)["']?`)

	// Python
	pyDefRe := regexp.MustCompile(`^(\s*)def\s+([a-zA-Z0-9_]+)\s*\(`)
	pyClassRe := regexp.MustCompile(`^(\s*)class\s+([a-zA-Z0-9_]+)`)

	// Java/C#
	javaClassRe := regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(?:static\s+)?(?:final\s+)?class\s+([a-zA-Z0-9_]+)`)
	javaMethodRe := regexp.MustCompile(`^\s*(?:public|protected|private)\s+(?:static\s+)?(?:final\s+)?[\w<>\[\]]+\s+([a-zA-Z0-9_]+)\s*\(`)

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)

		// 0. Update Levels
		if isBraceLang {
			braceLevel += strings.Count(text, "{")
			braceLevel -= strings.Count(text, "}")
			parenLevel += strings.Count(text, "(")
			parenLevel -= strings.Count(text, ")")
		}

		// 1. Check for Scope CLOSURE based on state
		if len(scopeStack) > 0 {
			closedCount := 0
			// Iterate backwards to close deeper scopes first
			for i := len(scopeStack) - 1; i >= 0; i-- {
				scope := &scopeStack[i]
				shouldClose := false

				if isBraceLang {
					if scope.CloseChar == "}" {
						if braceLevel <= scope.OpenLevel && strings.Contains(text, "}") {
							shouldClose = true
						}
					} else if scope.CloseChar == ")" {
						if parenLevel <= scope.OpenLevel && strings.Contains(text, ")") {
							shouldClose = true
						}
					}
				} else if isPython {
					if trimmed != "" {
						currentIndent := len(text) - len(strings.TrimLeft(text, " "))
						if currentIndent <= scope.Indent {
							shouldClose = true
						}
					}
				} else if isHtml {
					// HTML Closure: Look for </tag>
					if scope.Tag != "" && strings.Contains(strings.ToLower(text), "</"+strings.ToLower(scope.Tag)+">") {
						shouldClose = true
					}
				}

				if shouldClose {
					scope.Region.End = lineNum
					regions = append(regions, scope.Region)
					closedCount++
				} else {
					break
				}
			}
			if closedCount > 0 {
				scopeStack = scopeStack[:len(scopeStack)-closedCount]
			}
		}

		// 2. Check for NEW Region Start
		var name string
		var matched bool
		var newIndent int = 0
		var closeChar string = "}" // Default for brace langs
		var tagName string

		// Manual Markers (Override everything)
		if strings.Contains(text, "//") {
			matches := codeRe.FindStringSubmatch(text)
			if len(matches) == 3 {
				rawName := strings.TrimSpace(matches[2])
				// Clean up separators
				cleanName := strings.TrimFunc(rawName, func(r rune) bool {
					return r == '=' || r == '-' || r == ' ' || r == '\t'
				})

				if len(cleanName) > 0 {
					name = "📍 " + cleanName
					regions = append(regions, Region{Start: lineNum, End: lineNum, Name: name})
				}
				continue
			}
		}

		// Auto-Detect
		if isBraceLang {
			if ext == ".go" {
				if m := goFuncRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				} else if m := goTypeRe.FindStringSubmatch(text); len(m) > 1 {
					name = "📦 " + m[1] + " (" + m[2] + ")"
					matched = true
				} else if goImportRe.MatchString(text) {
					name = "📥 Imports"
					matched = true
					closeChar = ")"
				} else if goConstRe.MatchString(text) {
					name = "🧱 Const"
					matched = true
					closeChar = ")"
				} else if goVarRe.MatchString(text) {
					name = "🔨 Var"
					matched = true
					closeChar = ")"
				}
			} else if ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" {
				if m := jsFuncRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				} else if m := jsClassRe.FindStringSubmatch(text); len(m) > 1 {
					name = "📦 " + m[1]
					matched = true
				} else if m := jsArrowRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				} else if m := tsInterfaceRe.FindStringSubmatch(text); len(m) > 1 {
					name = "📄 " + m[1]
					matched = true
				} else if m := jsDescribeRe.FindStringSubmatch(text); len(m) > 1 {
					name = "🧪 " + m[1]
					matched = true
					closeChar = ")"
				} else if m := jsItRe.FindStringSubmatch(text); len(m) > 1 {
					name = "✓ " + m[1]
					matched = true
					closeChar = ")"
				} else if m := jsObjRe.FindStringSubmatch(text); len(m) > 1 {
					name = "🧱 " + m[1]
					matched = true
				} else if jsExportDefaultRe.MatchString(text) {
					name = "📦 default export"
					matched = true
				} else if m := jsRouteRe.FindStringSubmatch(text); len(m) > 2 {
					method := strings.ToUpper(m[1])
					path := m[2]
					name = "🛣️ " + method + " " + path
					matched = true
					closeChar = ")"
				} else if m := jsMethodRe.FindStringSubmatch(text); len(m) > 1 {
					candidate := m[1]
					if !jsKeywords[candidate] {
						name = "ƒ " + candidate
						matched = true
					}
				} else if m := jsPropFuncRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				} else if m := jsPropArrowRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				}
			} else if ext == ".java" || ext == ".cs" {
				if m := javaClassRe.FindStringSubmatch(text); len(m) > 1 {
					name = "📦 " + m[1]
					matched = true
				} else if m := javaMethodRe.FindStringSubmatch(text); len(m) > 1 {
					name = "ƒ " + m[1]
					matched = true
				}
			} else if ext == ".css" || ext == ".scss" || ext == ".less" {
				if strings.Contains(text, "{") {
					if m := cssRe.FindStringSubmatch(strings.TrimSpace(text)); len(m) > 1 {
						name = "🎨 " + strings.TrimSpace(m[1])
						matched = true
					}
				}
			}
		} else if isPython {
			if m := pyDefRe.FindStringSubmatch(text); len(m) > 1 {
				indentStr := m[1]
				name = "ƒ " + m[2]
				matched = true
				newIndent = len(indentStr)
			} else if m := pyClassRe.FindStringSubmatch(text); len(m) > 1 {
				indentStr := m[1]
				name = "📦 " + m[2]
				matched = true
				newIndent = len(indentStr)
			}
		} else if ext == ".md" {
			if m := mdRe.FindStringSubmatch(text); len(m) > 1 {
				name = m[2]
				regions = append(regions, Region{Start: lineNum, End: lineNum, Name: name})
				continue
			}
		}

		// Check for Dependencies
		depName := ""
		if ext == ".go" && depGoRe.MatchString(text) {
			m := depGoRe.FindStringSubmatch(text)
			if len(m) > 1 {
				depName = m[1]
			}
		} else if (ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx") && depJsRe.MatchString(text) {
			m := depJsRe.FindStringSubmatch(text)
			if len(m) > 1 {
				depName = m[1]
			}
		} else if isHtml {
			if m := depHtmlScriptRe.FindStringSubmatch(text); len(m) > 1 {
				depName = m[1]
			} else if m := depHtmlLinkRe.FindStringSubmatch(text); len(m) > 1 {
				if m[1] != "" {
					depName = m[1]
				} else {
					depName = m[2]
				}
			}
		} else if ext == ".css" && depCssRe.MatchString(text) {
			m := depCssRe.FindStringSubmatch(text)
			if len(m) > 1 {
				depName = m[1]
			}
		}

		if depName != "" {
			regions = append(regions, Region{Start: lineNum, End: lineNum, Name: "🔗 depends on: " + depName})
			// Do not mark as 'matched' for Scope stack, since dependencies are single lines
		}

		if isHtml {
			// HTML Tag Matching
			structuralTags := map[string]bool{
				"body": true, "div": true, "section": true, "article": true,
				"header": true, "footer": true, "nav": true, "main": true,
				"script": true, "style": true, "template": true,
			}

			if m := htmlTagRe.FindStringSubmatch(text); len(m) > 1 {
				openTag := m[1]

				if openTag != "" {
					// Opening Tag found
					lowerTag := strings.ToLower(openTag)
					if structuralTags[lowerTag] {
						// Extract ID or Class for nicer name
						name = "<" + openTag + ">"
						idRe := regexp.MustCompile(`id=["']([^"']+)["']`)
						classRe := regexp.MustCompile(`class=["']([^"']+)["']`)

						idMatch := idRe.FindStringSubmatch(text)
						classMatch := classRe.FindStringSubmatch(text)

						if len(idMatch) > 1 {
							name += " #" + idMatch[1]
						} else if len(classMatch) > 1 {
							fields := strings.Fields(classMatch[1])
							if len(fields) > 0 {
								name += " ." + fields[0]
							}
						}

						matched = true
						tagName = lowerTag
						closeChar = "</" + tagName + ">" // Not used for logic but logical
					}
				}
			}
		}

		if matched {
			// PUSH NEW SCOPE
			startParam := 0
			if isBraceLang {
				if closeChar == "}" {
					startParam = braceLevel
					if strings.Contains(text, "{") {
						startParam--
					}
				} else if closeChar == ")" {
					startParam = parenLevel
					if strings.Contains(text, "(") {
						startParam--
					}
				}
			} else if isPython {
				startParam = newIndent
			}

			// Determine Parent Context
			var parentName string
			for i := len(scopeStack) - 1; i >= 0; i-- {
				s := scopeStack[i]
				// Check if parent is Class, Object, or Interface
				if strings.Contains(s.Region.Name, "📦") || strings.Contains(s.Region.Name, "🧱") || strings.Contains(s.Region.Name, "📄") {
					// Extract clean name
					raw := s.Region.Name
					// Remove emoji and spaces
					parts := strings.Fields(raw)
					if len(parts) > 1 {
						parentName = parts[1]
					}
					break
				}
			}

			// If Method/Prop inside Class/Obj, prefix it
			if parentName != "" && (strings.HasPrefix(name, "ƒ") || strings.HasPrefix(name, "✓")) {
				// Strip icon from current name
				cleanCurrent := strings.TrimSpace(name[len("ƒ "):])
				if strings.HasPrefix(name, "✓") {
					cleanCurrent = strings.TrimSpace(name[len("✓ "):])
					name = "✓ " + parentName + " » " + cleanCurrent
				} else {
					name = "ƒ " + parentName + "." + cleanCurrent
				}
			}

			scopeStack = append(scopeStack, Scope{
				Region:    Region{Start: lineNum, Name: name},
				OpenLevel: startParam,
				Indent:    startParam,
				CloseChar: closeChar,
				Tag:       tagName,
			})
		}
	}

	// Close remaining scopes at end of file
	for i := len(scopeStack) - 1; i >= 0; i-- {
		scope := &scopeStack[i]
		scope.Region.End = lineNum
		regions = append(regions, scope.Region)
	}

	// Always generate a map
	mapPath := path + ".map.txt"
	var sb strings.Builder

	// --- METADATA HEADER ---
	sb.WriteString(fmt.Sprintf("File: %s\n", filepath.Base(path)))
	sb.WriteString(fmt.Sprintf("Path: %s\n", path))
	sb.WriteString(fmt.Sprintf("Size: %.2f KB\n", sizeKB))
	sb.WriteString(fmt.Sprintf("LOC: %d\n", lineNum))
	sb.WriteString(fmt.Sprintf("Modified: %s\n", modTime))
	sb.WriteString("--------------------------------------------------\n")

	// Sort regions by Start line
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].Start < regions[j].Start
	})

	// Post-Process: Extend single-line markers (like Headers or // Comments) to cover the block
	for i := 0; i < len(regions); i++ {
		// Only extend if it looks like a point-marker (Start == End)
		if regions[i].Start == regions[i].End {
			if i < len(regions)-1 {
				// Extend to next region's start - 1
				nextStart := regions[i+1].Start
				if nextStart > regions[i].Start {
					regions[i].End = nextStart - 1
				}
			} else {
				// Last region extends to end of file
				regions[i].End = lineNum
			}
		}
	}

	if len(regions) > 0 {
		for _, r := range regions {
			// Format: | Start | End | Name
			// Ensure End >= Start (sanity check)
			if r.End < r.Start {
				r.End = r.Start
			}
			sb.WriteString(fmt.Sprintf("| %4d | %4d | %s\n", r.Start, r.End, r.Name))
		}
	} else {
		// Default map for file without markers
		sb.WriteString(fmt.Sprintf("| %4d | %4d | (Entire File)\n", 1, lineNum))
	}

	err = os.WriteFile(mapPath, []byte(sb.String()), 0644)
	if err != nil {
		log.Printf("❌ Failed to write map: %s (%v)", filepath.Base(mapPath), err)
	}
}

// GenerateFolderMaps generates aggregated maps for all folders in the workspace
func GenerateFolderMaps(cfg config.Config, allFiles []string) {
	log.Println("📂 Generating Folder Maps (Covering ALL directories)...")

	// 1. Group watched files by Directory (for Level 1 lookup)
	watchedMap := make(map[string]map[string]bool)
	for _, f := range allFiles {
		dir := filepath.Dir(f)
		if watchedMap[dir] == nil {
			watchedMap[dir] = make(map[string]bool)
		}
		watchedMap[dir][f] = true
	}

	// 2. Identify ALL directories to scan (from Roots)
	targetDirs := make(map[string]bool)
	for _, rc := range cfg.Roots {
		absRoot, _ := filepath.Abs(rc.Path)
		filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if fs.ShouldIgnore(path, info, rc.IgnoredDirs) {
					return filepath.SkipDir
				}
				targetDirs[path] = true
			}
			return nil
		})
	}

	// 3. Process
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10)

	for dir := range targetDirs {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(d string) {
			defer wg.Done()
			defer func() { <-semaphore }()
			writeLevelMaps(d, watchedMap[d], cfg)
		}(dir)
	}
	wg.Wait()
}

func writeLevelMaps(dir string, watchedFiles map[string]bool, cfg config.Config) {
	dirName := filepath.Base(dir)
	nowStr := time.Now().Format(time.RFC3339)

	// --- LEVEL 0: INVENTORY (All Files) ---
	var sb0 strings.Builder
	sb0.WriteString(fmt.Sprintf("# LEVEL 0: INVENTORY - %s\n", dirName))
	sb0.WriteString(fmt.Sprintf("Path: %s\nGenerated: %s\n\n", dir, nowStr))
	sb0.WriteString("Do you need code structure? See: _level_1.map.txt\n")
	sb0.WriteString("Do you need subdirectories? See: _level_2.map.txt\n\n")
	sb0.WriteString("Name | Size | LOC | Modified\n")
	sb0.WriteString("---|---|---|---\n")

	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".map.txt") || fs.ShouldIgnoreName(e.Name(), nil) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			loc := fs.CountLines(filepath.Join(dir, e.Name()))
			modTime := info.ModTime().Format("2006-01-02 15:04")
			sb0.WriteString(fmt.Sprintf("%s | %d | %d | %s\n",
				e.Name(), info.Size(), loc, modTime))
		}
		os.WriteFile(filepath.Join(dir, "_level_0.map.txt"), []byte(sb0.String()), 0644)
	}

	// --- LEVEL 1: STRUCTURE (Watched Codes) ---
	var sb1 strings.Builder
	sb1.WriteString(fmt.Sprintf("# LEVEL 1: STRUCTURE - %s\n", dirName))
	sb1.WriteString(fmt.Sprintf("Path: %s\nGenerated: %s\n\n", dir, nowStr))
	sb1.WriteString("Need file inventory? See: _level_0.map.txt\n")
	sb1.WriteString("Need subdirectories? See: _level_2.map.txt\n\n")

	var sortedWatched []string
	for f := range watchedFiles {
		sortedWatched = append(sortedWatched, f)
	}
	sort.Strings(sortedWatched)

	for _, fPath := range sortedWatched {
		fName := filepath.Base(fPath)
		mapPath := fPath + ".map.txt"
		if _, err := os.Stat(mapPath); os.IsNotExist(err) {
			continue
		}

		content, err := os.ReadFile(mapPath)
		if err != nil {
			continue
		}

		sb1.WriteString(fmt.Sprintf("### %s\n", fName))

		lines := strings.Split(string(content), "\n")
		var inHeader = true
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l == "" {
				continue
			}
			// Header Skipping Logic
			if inHeader {
				if strings.HasPrefix(l, "File:") || strings.HasPrefix(l, "Path:") || strings.HasPrefix(l, "Size:") || strings.HasPrefix(l, "LOC:") || strings.HasPrefix(l, "Modified:") {
					continue
				}
				if strings.HasPrefix(l, "-----") {
					inHeader = false
					continue
				}
				// Format check
				if strings.HasPrefix(l, "|") || (strings.Contains(l, "|") && unicode.IsDigit(rune(l[0]))) {
					inHeader = false // New or old format detected
				}
			}

			if inHeader {
				continue
			}

			// Parse content lines for re-formatting (or just copy them?)
			// Existing code parsed and re-formatted.
			// New Format: | Start | End | Name
			if strings.HasPrefix(l, "|") {
				parts := strings.SplitN(l[1:], "|", 3)
				if len(parts) >= 3 {
					start := strings.TrimSpace(parts[0])
					end := strings.TrimSpace(parts[1])
					name := strings.TrimSpace(parts[2])
					sb1.WriteString(fmt.Sprintf("| %4s | %4s | %s\n", start, end, name))
					continue
				}
			}
			// Fallback Old Format
			parts := strings.SplitN(l, "|", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				rng := strings.TrimSpace(parts[0])
				sb1.WriteString(fmt.Sprintf("| %4s | %4s | %s\n", rng, rng, name)) // Best effort for old format
			} else {
				sb1.WriteString(fmt.Sprintf("| %4s | %4s | %s\n", "?", "?", l))
			}
		}
		sb1.WriteString("\n")
	}
	os.WriteFile(filepath.Join(dir, "_level_1.map.txt"), []byte(sb1.String()), 0644)

	// --- LEVEL 2: HIERARCHY ---
	var sb2 strings.Builder
	sb2.WriteString(fmt.Sprintf("# LEVEL 2: HIERARCHY - %s\n", dirName))
	sb2.WriteString(fmt.Sprintf("Path: %s\nGenerated: %s\n\n", dir, nowStr))

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == dir {
			return nil
		}
		if fs.ShouldIgnoreName(info.Name(), nil) {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(dir, path)
		level := strings.Count(rel, string(os.PathSeparator))
		indent := strings.Repeat("  ", level)
		sb2.WriteString(fmt.Sprintf("%s- 📁 %s/\n", indent, info.Name()))

		return nil
	})
	os.WriteFile(filepath.Join(dir, "_level_2.map.txt"), []byte(sb2.String()), 0644)

	// --- LEVEL 3: DEEP STRUCTURE ---
	var sb3 strings.Builder
	sb3.WriteString(fmt.Sprintf("# LEVEL 3: DEEP STRUCTURE - %s\n", dirName))
	sb3.WriteString(fmt.Sprintf("Path: %s\nGenerated: %s\n\n", dir, nowStr))

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == dir {
			return nil
		}
		if fs.ShouldIgnoreName(info.Name(), nil) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		level := strings.Count(rel, string(os.PathSeparator))
		indent := strings.Repeat("  ", level)

		if info.IsDir() {
			sb3.WriteString(fmt.Sprintf("%s- 📁 %s/\n", indent, info.Name()))
		} else {
			mapPath := path + ".map.txt"
			if _, err := os.Stat(mapPath); err == nil {
				sb3.WriteString(fmt.Sprintf("%s- 📄 %s\n", indent, info.Name()))
				content, err := os.ReadFile(mapPath)
				if err == nil {
					lines := strings.Split(string(content), "\n")
					for _, l := range lines {
						l = strings.TrimSpace(l)
						parts := strings.SplitN(l, "|", 4) // Try pipe format
						if len(parts) >= 3 && strings.HasPrefix(l, "|") {
							// | Start | End | Name
							// Let's rely on simple split
							p := strings.Split(l, "|")
							if len(p) >= 4 {
								start := strings.TrimSpace(p[1])
								end := strings.TrimSpace(p[2])
								name := strings.TrimSpace(p[3])
								sb3.WriteString(fmt.Sprintf("%s    | %4s | %4s | %s\n", indent, start, end, name))
							}
						}
					}
				}
			} else if !strings.HasSuffix(info.Name(), ".map.txt") {
				sb3.WriteString(fmt.Sprintf("%s- %s\n", indent, info.Name()))
			}
		}
		return nil
	})
	os.WriteFile(filepath.Join(dir, "_level_3.map.txt"), []byte(sb3.String()), 0644)
}

// UpdateFolderMaps triggers generation for a specific directory
func UpdateFolderMaps(dir string, watchlist map[string]bool, cfg config.Config) {
	// Filter watchlist for this dir
	watchedInDir := make(map[string]bool)
	normDir := strings.ToLower(filepath.Clean(dir))

	for path := range watchlist {
		if strings.ToLower(filepath.Clean(filepath.Dir(path))) == normDir {
			watchedInDir[path] = true
		}
	}
	writeLevelMaps(dir, watchedInDir, cfg)
}

func findRoot(cfg config.Config, dir string) string {
	dir = filepath.Clean(dir)
	for _, r := range cfg.Roots {
		root, _ := filepath.Abs(r.Path)
		if strings.HasPrefix(strings.ToLower(dir), strings.ToLower(root)) {
			return root
		}
	}
	// Fallback: If not found, use the first root or dir itself
	if len(cfg.Roots) > 0 {
		root, _ := filepath.Abs(cfg.Roots[0].Path)
		return root
	}
	return dir
}

// ScanNew scans only files that are NOT in the existing watchlist
func ScanNew(cfg config.Config, existingWatchlist map[string]bool) []string {
	log.Println("🔍 Scanning for NEW files only...")
	var newFiles []string

	for _, rc := range cfg.Roots {
		absRoot, _ := filepath.Abs(rc.Path)
		filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if fs.ShouldIgnore(path, info, rc.IgnoredDirs) {
					return filepath.SkipDir
				}
				return nil
			}

			// Check Allowed Extension
			ext := strings.ToLower(filepath.Ext(path))
			isAllowed := false
			for _, allowed := range rc.AllowedExts {
				if allowed == ext {
					isAllowed = true
					break
				}
			}

			if isAllowed {
				normPath := strings.ToLower(filepath.Clean(path))
				if !existingWatchlist[normPath] {
					// Found a new file!
					GenerateMap(path)
					newFiles = append(newFiles, path)
				}
			}
			return nil
		})
	}
	log.Printf("✨ Found and mapped %d new files.", len(newFiles))
	return newFiles
}

// ScanExtension scans only files with the given extension (regardless of watchlist)
func ScanExtension(cfg config.Config, targetExt string) []string {
	log.Printf("🔍 Scanning for extension: %s", targetExt)
	var scanned []string
	targetExt = strings.ToLower(targetExt)
	if !strings.HasPrefix(targetExt, ".") {
		targetExt = "." + targetExt
	}

	for _, rc := range cfg.Roots {
		absRoot, _ := filepath.Abs(rc.Path)
		filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if fs.ShouldIgnore(path, info, rc.IgnoredDirs) {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext == targetExt {
				GenerateMap(path)
				scanned = append(scanned, path)
			}
			return nil
		})
	}
	log.Printf("✨ Mapped %d files with extension %s.", len(scanned), targetExt)
	return scanned
}

// ScanFull performs a deep clean and then scans all allowed files.
func ScanFull(cfg config.Config) []string {
	for _, rc := range cfg.Roots {
		absRoot, _ := filepath.Abs(rc.Path)
		DeepClean(absRoot)
	}
	// Pass empty watchlist to force re-scan of everything allowed
	return ScanNew(cfg, make(map[string]bool))
}
