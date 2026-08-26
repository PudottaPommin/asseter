# Asseter

[![Go Reference](https://pkg.go.dev/badge/github.com/pudottapommin/asseter.svg)](https://pkg.go.dev/github.com/pudottapommin/asseter)

A Go tool for managing and embedding static assets in your web applications. Asseter helps you copy, compress, and generate embed-backed code for serving static files.

## Requirements

- Go 1.24 or higher

## Installation

```sh
go get github.com/pudottapommin/asseter/cmd/asseter
# or 
go get -tool github.com/pudottapommin/asseter/cmd/asseter
```

## Usage

### Commands

#### `copy` - Copy assets to output directory

Copies static assets from a source directory to a distribution directory, with optional font copying from node_modules.

**Options:**

| Flag         | Default   | Description                                                              |
|--------------|-----------|--------------------------------------------------------------------------|
| `-cwd`       | `""`      | Working directory (where node_modules are located)                       |
| `-src`       | `"src"`   | Source directory for assets                                              |
| `-dist`      | `"dist"`  | Distribution directory for assets                                        |
| `-font-dist` | `"files"` | Fonts distribution directory (rooted from dist)                          |
| `-exclude`   | -         | Exclude paths by glob pattern (can be used multiple times)               |
| `-font`      | -         | Font source names to copy from node_modules (can be used multiple times) |

**Examples:**

```sh
# Basic copy from 'src' to 'dist'
asseter copy -src src -dist dist

# Copy assets and extract specific fonts from node_modules
asseter copy -src src -dist dist -font inter -font "roboto-mono"

# Copy assets but exclude source map files and typescript files
asseter copy -src src -dist dist -exclude "*.map" -exclude "*.ts"
```

#### `generate` - Generate bindata Go file

Generates Go code for serving static assets by compressing assets into a subfolder and embedding them via Go's `//go:embed` directive.

**Options:**

| Flag         | Default                | Description                                                |
|--------------|------------------------|------------------------------------------------------------|
| `-cwd`       | `""`                   | Working directory                                          |
| `-src`       | `"assets"`             | Directory containing assets to process                     |
| `-out`       | `"bindata.asseter.go"` | Output Go filename                                         |
| `-pkg`       | `"assets"`             | Package name for the generated Go file                     |
| `-embed-dir` | `"_embed"`             | Subdirectory name for embedded asset files                 |

**Examples:**

```sh
# Generate the asset filesystem from the 'dist' folder
asseter generate -src dist -out internal/assets/bindata.asseter.go -pkg assets

# Generate with a custom embed directory name
asseter generate -src dist -out internal/assets/bindata.asseter.go -pkg assets -embed-dir _embed
```

## Workflow

A typical workflow combines both commands, often orchestrated via a `go:generate` directive or a build script:

```sh
# 1. Gather all assets into a staging 'dist' folder, excluding source maps
asseter copy -src frontend/public -dist dist -exclude "*.map" -font inter

# 2. Generate the compressed, embedded Go filesystem
asseter generate -src dist -out internal/assets/bindata.asseter.go -pkg assets
```

### Using the generated assets in your Go code

Build with the `bindata` build tag (`go build -tags bindata .`):

```go
package main

import (
	"net/http"
	"your-project/internal/assets" // Import the generated package
)

func main() {
	mux := http.NewServeMux()
	
	// Serve the embedded static files.
	// The generated assets.Assets implements fs.FS and gracefully handles directories and compression.
	fileServer := http.FileServer(http.FS(assets.Assets))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
	
	http.ListenAndServe(":8080", mux)
}
```
