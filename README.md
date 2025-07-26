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

[//]: # (**Examples:**)

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

[//]: # (**Examples:**)

[//]: # (## Workflow)

[//]: # (A typical workflow combines both commands:)
