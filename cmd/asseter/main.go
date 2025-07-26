package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pudottapommin/asseter"
)

// PLAN
// Step 1: clean output directory
// Step 2: copy assets
// Step 3: copy fonts if defined
// Step 4: generate hash if needed
// Step 5: generate assets_gen.go

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: asseter <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  copy     Copy assets to output directory")
		fmt.Println("  gen   Generate assets_gen.go file")
		fmt.Println("  hash     Version assets files in directory")
		os.Exit(1)
	}

	ctx := context.Background()
	switch os.Args[1] {
	case "copy":
		t := time.Now()
		cmd, err := newCopyHandlerFromCLI()
		if err != nil {
			log.Fatalln(err)
		}
		if err = cmd.Run(ctx); err != nil {
			log.Fatalln(err)
		}
		fmt.Println("Elapsed ", time.Since(t))
	case "gen":
		t := time.Now()
		cmd, err := newAssetsHandlerFromCLI()
		if err != nil {
			log.Fatalln(err)
		}
		if err = cmd.Run(ctx); err != nil {
			log.Fatalln(err)
		}
		fmt.Println("Elapsed ", time.Since(t))
	}
}

func newCopyHandlerFromCLI() (cmd *asseter.CopyHandler, err error) {
	o := asseter.CopyOptions{}
	// CWD, SRC, DIST
	flagSet := flag.NewFlagSet("copy", flag.ExitOnError)
	flagSet.StringVar(&o.Cwd, "cwd", "", "Working directory ( where are node_modules )")
	flagSet.StringVar(&o.SrcDir, "src", "src", "Source directory for assets")
	flagSet.StringVar(&o.DistDir, "dist", "dist", "Dist directory for assets")
	flagSet.StringVar(&o.DistFontsDir, "fontDist", "files", "Fonts dist directory rooted from dist")
	flagSet.Var(&o.Exclude, "exclude", "Exclude paths by glob")
	flagSet.Var(&o.Fonts, "font", "Font source font names to copy from node_modules")
	if err = flagSet.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	cmd, err = asseter.NewCopyHandler(o)
	if err != nil {
		return nil, err
	}
	return
}

func newAssetsHandlerFromCLI() (cmd *asseter.AssetsHandler, err error) {
	o := asseter.AssetsOptions{}
	flagSet := flag.NewFlagSet("gen", flag.ExitOnError)
	flagSet.StringVar(&o.Cwd, "cwd", "", "Working directory ( where are node_modules )")
	flagSet.StringVar(&o.SrcDir, "src", "dist", "Directory for assets")
	flagSet.StringVar(&o.DistDir, "dist", "assets", "Directory where assets will be generated")
	flagSet.StringVar(&o.Pkg, "pkg", "assets", "Package name for generated file")
	flagSet.StringVar(&o.Server, "server", "http", "Binding for HTTP server (http,gin)")
	flagSet.StringVar(&o.UrlPrefix, "urlPrefix", "/static", "URL path to prepend to all assets")
	flagSet.BoolVar(&o.IsEmbed, "embed", false, "Flag to embed assets into binary")
	flagSet.BoolVar(&o.ShouldHash, "hash", false, "Flag to hash assets files")
	flagSet.Var(&o.Exclude, "exclude", "Exclude paths by glob")
	if err = flagSet.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	cmd, err = asseter.NewAssetsHandler(o)
	if err != nil {
		return nil, err
	}
	return
}
