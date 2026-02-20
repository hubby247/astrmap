<div align="center">
  <h1>🗺️ AstrMap</h1>
  <p><b>UNIX philosophy for the AI era. Ditch the RAG. Give your LLM a map.</b></p>
  <p>RAG is a black box that loses context. Sending raw code burns tokens. AstrMap gives your AI agent deterministic, lightning-fast context of your entire codebase by treating structure as pure, parseable text.</p>
</div>

---

## 🛑 The Problem: RAG is a Black Box (and raw code burns tokens)
If you've built AI agents or pasted code into Claude, you know the pain:
1. **RAG guesses:** Vector databases chop your pristine architecture into floating chunks and *guess* what's relevant. When they guess wrong, the AI breaks your code.
2. **Raw Code is Noisy:** Dumping 50 files into context causes "Lost in the Middle" hallucination. 
3. **Complexity:** Setting up embeddings, chunks, and vector stores for a codebase is fundamentally un-UNIX.

## ⚡ The Solution: KISS (Keep It Simple, Stupid) 
**AstrMap** is a wildly fast, single-binary Go CLI tool. Instead of complex embeddings, it relies on simple, deterministic maps. 

AstrMap parses your AST (Abstract Syntax Tree) in milliseconds and generates human- and AI-readable index files (`.map.txt`). It turns a 10,000-line monolithic repository into a concise, 200-line index. Just like *everything is a file* in UNIX, to AstrMap, *your architecture is just a text file*.

Your AI reads the Map (costing almost zero tokens) and independently navigates the exact files and lines it needs. No vectors. No API keys. No bullshit.

Your AI reads the Map (costing basically zero tokens) and then knows *exactly* which specific files and line numbers it actually needs to look at.

## 🚀 Features (Free CLI)
- **Instant Speed:** Written in Go. Parses hundreds of files in milliseconds.
- **Multi-Language Support:** Natively unwraps Go, Python, JavaScript/TypeScript, HTML, and CSS.
- **Level-Based Details:** Generates hierarchical overviews (`_level_1` for folders, `_level_3` for inline functions/classes).
- **Markdown Conscious:** Treats `# Headers` in your documentation as distinct, searchable code regions.
- **100% Local & Secure:** No cloud, no vectors, no API keys required.

## 📖 How to use it with an AI
1. Run `$ astrmap scan ./my-project`
2. Drop `_level_3.map.txt` into ChatGPT, Claude, or Cursor.
3. Prompt: *"Here is the map of my codebase. I need to add a password reset feature. Based on this map, which files should we modify?"*
4. Watch the AI navigate your architecture flawlessly.

## 💻 Installation
```bash
go install github.com/hubby247/astrmap@latest
```

## 🌟 AstrMap PRO (Coming Soon)
Need more power? The upcoming PRO version features:
- **Zero-Latency Live Watcher:** Runs silently in the background, updating maps the millisecond you hit `Save`.
- **Desktop UI Visualizer:** Drag-and-drop your repo into a beautiful Dashboard.
- **Dependency Tracking:** Tracks cross-file imports (`import`, `require`) and builds a visual dependency web.
- **One-Click AI Context Copy:** Copy the perfect, token-optimized context payload straight to your clipboard.

*(Subscribe/Star this repo to get notified when PRO drops!)*
