# Asseter

[![Go Reference](https://pkg.go.dev/badge/github.com/pudottapommin/asseter.svg)](https://pkg.go.dev/github.com/pudottapommin/asseter)

A Go tool for managing and embedding static assets in your web applications. Asseter helps you copy, version, and
generate code for serving static files with support for both standard `http` and `gin` web frameworks.

## Requirements

- Go 1.24 or higher

## Installation

```
go get github.com/pudottapommin/asseter/cmd/asseter
# or 
go get -tool github.com/pudottapommin/asseter/cmd/asseter
```

## Usage

### Commands

#### `copy` - Copy assets to output directory

Copies static assets from a source directory to a distribution directory, with optional font copying from node_modules.

**Usage:**

**Options:**

| Flag        | Default   | Description                                                              |
|-------------|-----------|--------------------------------------------------------------------------|
| `-cwd`      | `""`      | Working directory (where node_modules are located)                       |
| `-src`      | `"src"`   | Source directory for assets                                              |
| `-dist`     | `"dist"`  | Distribution directory for assets                                        |
| `-fontDist` | `"files"` | Fonts distribution directory (rooted from dist)                          |
| `-exclude`  | -         | Exclude paths by glob pattern (can be used multiple times)               |
| `-font`     | -         | Font source names to copy from node_modules (can be used multiple times) |

**Examples:**

```sh
# Basic copy from 'src' to 'dist'
asseter copy -src src -dist dist

# Copy assets and extract specific fonts from node_modules
asseter copy -src src -dist dist -font inter -font "roboto-mono"

# Copy assets but exclude source map files and typescript files
asseter copy -src src -dist dist -exclude "*.map" -exclude "*.ts"
```

#### `gen` - Generate assets_gen.go file

Generates Go code for serving static assets with optional embedding and versioning support.

**Usage:**

**Options:**

| Flag         | Default     | Description                                                |
|--------------|-------------|------------------------------------------------------------|
| `-cwd`       | `""`        | Working directory (where node_modules are located)         |
| `-src`       | `"dist"`    | Directory containing assets to process                     |
| `-dist`      | `"assets"`  | Directory where Go file will be generated                  |
| `-pkg`       | `"assets"`  | Package name for the generated Go file                     |
| `-server`    | `"http"`    | HTTP server binding (`http` or `gin`)                      |
| `-urlPrefix` | `"/static"` | URL path prefix for all assets                             |
| `-embed`     | `false`     | Embed assets into the binary                               |
| `-hash`      | `false`     | Generate versioned asset filenames with hashes             |
| `-exclude`   | -           | Exclude paths by glob pattern (can be used multiple times) |

**Examples:**

```sh
# Generate the asset filesystem from the 'dist' folder
asseter gen -src dist -dist internal/assets -pkg assets

# Generate embedded, hashed assets for cache-busting (standard http)
asseter gen -src dist -dist internal/assets -pkg assets -embed -hash -urlPrefix "/static"
```

## Workflow

A typical workflow combines both commands, often orchestrated via a `go:generate` directive or a build script:

```sh
# 1. Gather all assets into a staging 'dist' folder, excluding source maps
asseter copy -src frontend/public -dist dist -exclude "*.map" -font inter

# 2. Generate the compressed, embedded Go filesystem
asseter gen -src dist -dist internal/assets -pkg assets -embed -hash -urlPrefix "/static"
```

### Using the generated assets in your Go code

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
