package asseter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"
)

type (
	CopyHandler struct {
		cwd          string
		srcDir       string
		distDir      string
		distFontsDir string
		urlBase      string
		exclude      FileMatchFlag
		fonts        FontSourceFontsFlag

		isSetup bool
		filesCh chan string
	}
	CopyOptions struct {
		Cwd          string
		SrcDir       string
		DistDir      string
		Exclude      FileMatchFlag
		Fonts        FontSourceFontsFlag
		DistFontsDir string
	}
)

func NewCopyHandler(opts CopyOptions) (*CopyHandler, error) {
	cmd := &CopyHandler{
		cwd:          opts.Cwd,
		srcDir:       opts.SrcDir,
		distDir:      opts.DistDir,
		distFontsDir: opts.DistFontsDir,
		urlBase:      "",
		exclude:      opts.Exclude,
		fonts:        opts.Fonts,
		isSetup:      true,
		filesCh:      nil,
	}
	if err := cmd.normalizeDirPaths(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func (cmd *CopyHandler) Run(ctx context.Context) error {
	if !cmd.isSetup {
		return errors.New("CopyHandler is not setup")
	}

	cmd.filesCh = make(chan string)
	g, gctx := errgroup.WithContext(ctx)
	if err := cmd.cleanDest(gctx); err != nil {
		return err
	}
	g.Go(func() error {
		return cmd.walkSources(gctx)
	})
	g.Go(func() error {
		return cmd.walkFonts(gctx)
	})
	go func() {
		_ = g.Wait()
		close(cmd.filesCh)
	}()

	paths := make([]string, 0, 12) //nolint:mnd // just a arbitrary number
	for fc := range cmd.filesCh {
		paths = append(paths, fc)
	}
	if err := g.Wait(); err != nil {
		return err
	}

	buffer := new(bytes.Buffer)
	for _, p := range paths {
		if err := cmd.copyFile(buffer, p); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *CopyHandler) resolveDestinationPath(p string, name string) (string, error) {
	if matched, _ := filepath.Match("*.woff2", name); matched {
		if err := os.MkdirAll(cmd.distFontsDir, 0o750); err != nil {
			return "", err
		}
		return filepath.Join(cmd.distFontsDir, filepath.Base(p)), nil
	}

	rel, err := filepath.Rel(cmd.srcDir, p)
	var distFilePath string
	if err != nil || strings.HasPrefix(rel, "..") {
		distFilePath = filepath.Join(cmd.distDir, filepath.Base(p))
	} else {
		distFilePath = filepath.Join(cmd.distDir, rel)
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(distFilePath), 0o750); mkdirErr != nil {
		return "", mkdirErr
	}
	return distFilePath, nil
}

func (cmd *CopyHandler) copyFile(buffer *bytes.Buffer, p string) error {
	defer buffer.Reset()
	src, err := os.OpenFile(p, os.O_RDONLY, 0o600)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}

	distFilePath, err := cmd.resolveDestinationPath(p, info.Name())
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(distFilePath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, info.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func (cmd *CopyHandler) walkSources(ctx context.Context) error {
	return filepath.WalkDir(cmd.srcDir, func(p string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		p = filepath.ToSlash(p)

		if p == cmd.distDir || strings.HasPrefix(p, cmd.distDir+"/") {
			if de.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if p == cmd.srcDir {
			return nil
		}

		if cmd.exclude.Match(p) {
			if de.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if de.IsDir() {
			return nil
		}

		select {
		case cmd.filesCh <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func (cmd *CopyHandler) walkFonts(ctx context.Context) error {
	if len(cmd.fonts) == 0 {
		return nil
	}
	nodeModulesDir := filepath.ToSlash(path.Join(cmd.cwd, "node_modules"))
	return filepath.WalkDir(nodeModulesDir, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		p = filepath.ToSlash(p)
		if matched := strings.Contains(p, "@fontsource"); !matched {
			return nil
		}
		if matched := cmd.fonts.Match(de.Name()); !matched {
			return nil
		}
		select {
		case cmd.filesCh <- p:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func (cmd *CopyHandler) normalizeDirPaths() error {
	if cmd.cwd == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cmd.cwd = cwd
	}
	cmd.cwd = filepath.ToSlash(filepath.Clean(cmd.cwd))

	if !filepath.IsAbs(cmd.srcDir) {
		cmd.srcDir = filepath.Join(cmd.cwd, cmd.srcDir)
	}
	cmd.srcDir = filepath.ToSlash(filepath.Clean(cmd.srcDir))

	if !filepath.IsAbs(cmd.distDir) {
		cmd.distDir = filepath.Join(cmd.cwd, cmd.distDir)
	}
	cmd.distDir = filepath.ToSlash(filepath.Clean(cmd.distDir))

	if !filepath.IsAbs(cmd.distFontsDir) {
		cmd.distFontsDir = filepath.Join(cmd.distDir, cmd.distFontsDir)
	}
	cmd.distFontsDir = filepath.ToSlash(filepath.Clean(cmd.distFontsDir))

	return nil
}

func (cmd *CopyHandler) cleanDest(ctx context.Context) error {
	if _, err := os.Stat(cmd.distDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.Mkdir(cmd.distDir, 0o750)
		}
		return err
	}
	dirs := make([]string, 0, 4) //nolint:mnd //noop
	filesToRemove := make([]string, 0)
	if err := filepath.WalkDir(cmd.distDir, func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		p = filepath.ToSlash(p)
		if de.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		if !cmd.exclude.Match(p) || cmd.fonts.Match(filepath.Base(p)) {
			filesToRemove = append(filesToRemove, p)
		}
		return nil
	}); err != nil {
		return err
	}

	for _, p := range filesToRemove {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}

	return cmd.cleanEmptyDirs(ctx, dirs)
}

func (cmd *CopyHandler) cleanEmptyDirs(ctx context.Context, dirs []string) error {
	for _, d := range slices.Backward(dirs) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			entries, err := os.ReadDir(d)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				if rmErr := os.Remove(d); rmErr != nil {
					return rmErr
				}
			}
		}
	}
	return nil
}
