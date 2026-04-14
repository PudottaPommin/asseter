package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pudottapommin/asseter"
	"github.com/urfave/cli/v3"
)

var version = "dev"

func main() {
	app := newApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newApp() *cli.Command {
	return &cli.Command{
		Name:    "asseter",
		Usage:   "Asset management tool",
		Version: version,
		Commands: []*cli.Command{
			{
				Name:   "copy",
				Usage:  "Copy assets to output directory",
				Before: validateCopyFlags,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "cwd",
						Usage: "Working directory ( where are node_modules )",
					},
					&cli.StringFlag{
						Name:  "src",
						Value: "src",
						Usage: "Source directory for assets",
					},
					&cli.StringFlag{
						Name:  "dist",
						Value: "dist",
						Usage: "Dist directory for assets",
					},
					&cli.StringFlag{
						Name:  "font-dist",
						Value: "files",
						Usage: "Fonts dist directory rooted from dist",
					},
					&cli.StringSliceFlag{
						Name:  "exclude",
						Usage: "Exclude paths by glob",
					},
					&cli.StringSliceFlag{
						Name:  "font",
						Usage: "Font source font names to copy from node_modules",
					},
				},
				Action: handleCopy,
			},
			{
				Name:   "generate",
				Usage:  "Generate bindata assets",
				Before: validateGenerateFlags,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "cwd",
						Usage: "Working directory",
					},
					&cli.StringFlag{
						Name:  "src",
						Value: "assets",
						Usage: "Directory for assets",
					},
					&cli.StringFlag{
						Name:  "out",
						Value: "bindata.asseter.go",
						Usage: "Output filename",
					},
					&cli.StringFlag{
						Name:  "pkg",
						Value: "assets",
						Usage: "Package name for generated file",
					},
				},
				Action: handleGen,
			},
		},
	}
}

func validateCopyFlags(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	absCwd, err := resolveCwd(cmd)
	if err != nil {
		return ctx, err
	}

	// Resolve and validate src
	srcPath := cmd.String("src")
	absSrc, err := resolvePath(absCwd, srcPath)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve src path: %w", err)
	}
	if info, err := os.Stat(absSrc); err != nil {
		return ctx, fmt.Errorf("failed to access src path: %w", err)
	} else if !info.IsDir() {
		return ctx, fmt.Errorf("src must be a directory: %s", absSrc)
	}
	if err = cmd.Set("src", absSrc); err != nil {
		return ctx, err
	}

	// Resolve and validate dist
	distPath := cmd.String("dist")
	absDist, err := resolvePath(absCwd, distPath)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve dist path: %w", err)
	}
	if err = cmd.Set("dist", absDist); err != nil {
		return ctx, err
	}

	// Resolve font-dist relative to dist
	fontDistPath := cmd.String("font-dist")
	absFontDist, err := resolvePath(absDist, fontDistPath)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve font-dist path: %w", err)
	}
	if err = cmd.Set("font-dist", absFontDist); err != nil {
		return ctx, err
	}

	return ctx, nil
}

func validateGenerateFlags(ctx context.Context, cmd *cli.Command) (context.Context, error) {
	absCwd, err := resolveCwd(cmd)
	if err != nil {
		return ctx, err
	}

	// Resolve and validate src
	srcPath := cmd.String("src")
	absSrc, err := resolvePath(absCwd, srcPath)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve src path: %w", err)
	}
	if info, err := os.Stat(absSrc); err != nil {
		return ctx, fmt.Errorf("failed to access src path: %w", err)
	} else if !info.IsDir() {
		return ctx, fmt.Errorf("src must be a directory: %s", absSrc)
	}
	if err = cmd.Set("src", absSrc); err != nil {
		return ctx, err
	}

	// Resolve and validate out
	outPath := cmd.String("out")
	absOut, err := resolvePath(absCwd, outPath)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve out path: %w", err)
	}
	if info, err := os.Stat(absOut); err == nil && info.IsDir() {
		absOut = filepath.Join(absOut, "bindata.asseter.go")
	}

	outDir := filepath.Dir(absOut)
	if info, err := os.Stat(outDir); err != nil {
		return ctx, fmt.Errorf("output directory does not exist: %s", outDir)
	} else if !info.IsDir() {
		return ctx, fmt.Errorf("output path parent is not a directory: %s", outDir)
	}
	if err = cmd.Set("out", absOut); err != nil {
		return ctx, err
	}

	return ctx, nil
}

func resolveCwd(cmd *cli.Command) (string, error) {
	cwdPath := cmd.String("cwd")
	if cwdPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		cwdPath = wd
	}
	absCwd, err := filepath.Abs(cwdPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for cwd: %w", err)
	}
	if err = cmd.Set("cwd", absCwd); err != nil {
		return "", err
	}
	return absCwd, nil
}

func resolvePath(base, path string) (string, error) {
	p := filepath.Clean(path)
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return filepath.Abs(p)
}

func handleCopy(ctx context.Context, cmd *cli.Command) error {
	t := time.Now()
	o := asseter.CopyOptions{
		Cwd:          cmd.String("cwd"),
		SrcDir:       cmd.String("src"),
		DistDir:      cmd.String("dist"),
		DistFontsDir: cmd.String("font-dist"),
		Exclude:      asseter.FileMatchFlag(cmd.StringSlice("exclude")),
		Fonts:        asseter.FontSourceFontsFlag(cmd.StringSlice("font")),
	}

	handler, err := asseter.NewCopyHandler(o)
	if err != nil {
		return err
	}
	if err = handler.Run(ctx); err != nil {
		return err
	}
	fmt.Printf("Successfully copied assets in %v\n", time.Since(t))
	return nil
}

func handleGen(_ context.Context, cmd *cli.Command) error {
	t := time.Now()
	o := asseter.AssetsFsOptions{
		Src: cmd.String("src"),
		Out: cmd.String("out"),
		Pkg: cmd.String("pkg"),
	}

	handler, err := asseter.NewAssetsFsHandler(o)
	if err != nil {
		return err
	}
	if err = handler.Run(); err != nil {
		return err
	}
	fmt.Printf("Successfully generated assets in %v\n", time.Since(t))
	return nil
}
