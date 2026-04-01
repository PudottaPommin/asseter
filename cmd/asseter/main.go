package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pudottapommin/asseter"
	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "asseter",
		Usage: "Asset management tool",
		Commands: []*cli.Command{
			{
				Name:  "copy",
				Usage: "Copy assets to output directory",
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
						Name:  "fontDist",
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
				Name:  "generate",
				Usage: "Generate bindata assets",
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
				Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
					// Validate and clean cwd
					cwdPath := cmd.String("cwd")
					if cwdPath == "" {
						wd, err := os.Getwd()
						if err != nil {
							return ctx, fmt.Errorf("failed to get working directory: %w", err)
						}
						cwdPath = wd
					}
					cwdPath = filepath.Clean(cwdPath)
					absCwd, err := filepath.Abs(cwdPath)
					if err != nil {
						return ctx, fmt.Errorf("failed to get absolute path for cwd: %w", err)
					}
					if err = cmd.Set("cwd", absCwd); err != nil {
						return ctx, fmt.Errorf("failed to set cwd: %w", err)
					}

					// Validate and clean src
					srcPath := cmd.String("src")
					srcPath = filepath.Clean(srcPath)
					if !filepath.IsAbs(srcPath) {
						srcPath = filepath.Join(absCwd, srcPath)
					}
					absSrc, err := filepath.Abs(srcPath)
					if err != nil {
						return ctx, fmt.Errorf("failed to get absolute path for src: %w", err)
					}
					info, err := os.Stat(absSrc)
					if err != nil {
						return ctx, fmt.Errorf("failed to access src path: %w", err)
					}
					if !info.IsDir() {
						return ctx, fmt.Errorf("src must be a directory, not a file: %s", absSrc)
					}
					if err = cmd.Set("src", absSrc); err != nil {
						return ctx, fmt.Errorf("failed to set src: %w", err)
					}

					// Validate and clean out
					outPath := cmd.String("out")
					outPath = filepath.Clean(outPath)
					if !filepath.IsAbs(outPath) {
						outPath = filepath.Join(absCwd, outPath)
					}
					absOut, err := filepath.Abs(outPath)
					if err != nil {
						return ctx, fmt.Errorf("failed to get absolute path for out: %w", err)
					}
					if outInfo, err := os.Stat(absOut); err == nil && outInfo.IsDir() {
						absOut = filepath.Join(absOut, "bindata.asseter.go")
					}
					outDir := filepath.Dir(absOut)
					if dirInfo, err := os.Stat(outDir); err != nil {
						return ctx, fmt.Errorf("output directory does not exist: %s", outDir)
					} else if !dirInfo.IsDir() {
						return ctx, fmt.Errorf("output directory path is not a directory: %s", outDir)
					}
					if err = cmd.Set("out", absOut); err != nil {
						return ctx, fmt.Errorf("failed to set out: %w", err)
					}

					return ctx, nil
				},
				Action: handleGen,
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Panicf("Error: %v\n", err)
	}
}

func handleCopy(ctx context.Context, cmd *cli.Command) error {
	t := time.Now()
	o := asseter.CopyOptions{
		Cwd:          cmd.String("cwd"),
		SrcDir:       cmd.String("src"),
		DistDir:      cmd.String("dist"),
		DistFontsDir: cmd.String("fontDist"),
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
