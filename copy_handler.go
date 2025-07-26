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

func NewCopyHandler(opts CopyOptions) (cmd *CopyHandler, err error) {
	cmd = &CopyHandler{
		cwd:          opts.Cwd,
		srcDir:       opts.SrcDir,
		distDir:      opts.DistDir,
		exclude:      opts.Exclude,
		fonts:        opts.Fonts,
		distFontsDir: opts.DistFontsDir,
		isSetup:      true,
	}
	if err = cmd.normalizeDirPaths(); err != nil {
		return nil, err
	}
	return
}

func (cmd *CopyHandler) Run(ctx context.Context) (err error) {
	if !cmd.isSetup {
		return errors.New("CopyHandler is not setup")
	}

	cmd.filesCh = make(chan string)
	g, gctx := errgroup.WithContext(ctx)
	if err = cmd.cleanDest(gctx); err != nil {
		return
	}
	g.Go(func() error {
		return cmd.walkSources(gctx)
	})
	g.Go(func() error {
		return cmd.walkFonts(gctx)
	})
	go func() {
		g.Wait()
		close(cmd.filesCh)
	}()

	paths := make([]string, 0, 10)
	for fc := range cmd.filesCh {
		paths = append(paths, fc)
	}
	if err = g.Wait(); err != nil {
		return err
	}

	buffer := new(bytes.Buffer)
	for _, p := range paths {
		if err = cmd.copyFile(buffer, p); err != nil {
			return err
		}
	}

	return nil
}

func (cmd *CopyHandler) copyFile(buffer *bytes.Buffer, p string) error {
	defer buffer.Reset()
	src, err := os.OpenFile(p, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	distFilePath := filepath.Join(cmd.distDir, filepath.Base(p))
	if matched, _ := filepath.Match("*.woff2", info.Name()); matched {
		if _, err = os.Stat(cmd.distFontsDir); err != nil {
			if os.IsNotExist(err) {
				if err = os.Mkdir(cmd.distFontsDir, 0755); err != nil {
					return err
				}
			}
		}
		distFilePath = filepath.Join(cmd.distFontsDir, filepath.Base(p))
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
		if strings.HasPrefix(p, cmd.distDir) || p == cmd.srcDir {
			return nil
		}
		if cmd.exclude.Match(p) {
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

func (cmd *CopyHandler) walkFonts(ctx context.Context) (err error) {
	if len(cmd.fonts) > 0 {
		nodeModulesDir := filepath.ToSlash(path.Join(cmd.cwd, "node_modules"))
		err = filepath.WalkDir(nodeModulesDir, func(p string, de fs.DirEntry, err error) error {
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
	return
}

func (cmd *CopyHandler) normalizeDirPaths() (err error) {
	if cmd.cwd == "" {
		cmd.cwd, err = os.Getwd()
		if err != nil {
			return
		}
		cmd.cwd = filepath.ToSlash(path.Clean(cmd.cwd))
	}
	cmd.srcDir = filepath.ToSlash(path.Join(cmd.cwd, cmd.srcDir))
	cmd.distDir = filepath.ToSlash(path.Join(cmd.cwd, cmd.distDir))
	cmd.distFontsDir = filepath.ToSlash(path.Join(cmd.distDir, cmd.distFontsDir))
	return
}

func (cmd *CopyHandler) cleanDest(ctx context.Context) (err error) {
	if _, err = os.Stat(cmd.distDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.Mkdir(cmd.distDir, 0755)
		}
		return err
	}
	dirs := make([]string, 0, 4)
	err = filepath.WalkDir(cmd.distDir, func(p string, de fs.DirEntry, err error) error {
		p = filepath.ToSlash(p)
		if de.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		if !cmd.exclude.Match(p) || cmd.fonts.Match(filepath.Base(p)) {
			return os.Remove(p)
		}
		return nil
	})
	if err == nil {
		for _, d := range dirs {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				switch e, err := os.ReadDir(d); {
				case err != nil:
					return err
				case len(e) == 0:
					if err = os.Remove(d); err != nil {
						return err
					}
				}

			}
		}
	}
	return
}
