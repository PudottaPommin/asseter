package asseter

import (
	"bytes"
	"context"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"go/format"
	"hash/crc32"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

type (
	templateFileInfo struct {
		Key  string
		Path string
		Url  string
	}

	templateModel struct {
		Server    string
		DistDir   string
		Pkg       string
		Files     []*templateFileInfo
		IsEmbed   bool
		UrlPrefix string
	}
)

const crcPol = 0xD5828281

var (
	//go:embed *.gotmpl
	tmplFS embed.FS
	tmpl   = template.Must(template.ParseFS(tmplFS, "*.gotmpl"))
)

type (
	AssetsHandler struct {
		cwd        string
		srcDir     string
		distDir    string
		pkg        string
		urlPrefix  string
		server     string
		exclude    FileMatchFlag
		isEmbed    bool
		shouldHash bool

		isSetup bool
		hasher  *crc32.Table
		filesCh chan [2]string
	}
	AssetsOptions struct {
		Cwd        string
		SrcDir     string
		DistDir    string
		Pkg        string
		UrlPrefix  string
		Server     string
		Exclude    FileMatchFlag
		IsEmbed    bool
		ShouldHash bool
	}
)

func NewAssetsHandler(opts AssetsOptions) (cmd *AssetsHandler, err error) {
	cmd = &AssetsHandler{
		cwd:        opts.Cwd,
		srcDir:     opts.SrcDir,
		distDir:    opts.DistDir,
		pkg:        opts.Pkg,
		urlPrefix:  opts.UrlPrefix,
		server:     opts.Server,
		exclude:    opts.Exclude,
		isEmbed:    opts.IsEmbed,
		shouldHash: opts.ShouldHash,
		isSetup:    true,
	}
	if err = cmd.normalizeDirPaths(); err != nil {
		return nil, err
	}
	if cmd.shouldHash {
		cmd.hasher = crc32.MakeTable(crcPol)
	}
	return
}

func (cmd *AssetsHandler) Run(ctx context.Context) (err error) {
	if !cmd.isSetup {
		return errors.New("AssetsHandler is not setup")
	}

	cmd.filesCh = make(chan [2]string)
	hasher := crc32.MakeTable(crcPol)
	_ = hasher
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(cmd.filesCh)
		return cmd.walkAssets(gctx)
	})
	g.Go(func() error {
		return cmd.renderTemplate(gctx)
	})

	err = g.Wait()
	return
}

func (cmd *AssetsHandler) walkAssets(ctx context.Context) (err error) {
	return filepath.WalkDir(cmd.srcDir, func(p string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			return nil
		}

		p = filepath.ToSlash(p)
		if cmd.exclude.Match(p) {
			return nil
		}

		h, err := cmd.hashFile(p)
		if err != nil {
			return err
		}

		select {
		case cmd.filesCh <- [2]string{p, h}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})
}

func (cmd *AssetsHandler) renderTemplate(_ context.Context) (err error) {
	dd := filepath.Base(cmd.srcDir)
	if !cmd.isEmbed {
		r, _ := filepath.Rel(cmd.cwd, cmd.srcDir)
		dd = filepath.ToSlash(r)
	}
	model := templateModel{
		DistDir:   dd,
		Pkg:       cmd.pkg,
		UrlPrefix: cmd.urlPrefix,
		IsEmbed:   cmd.isEmbed,
		Server:    cmd.server,
		Files:     make([]*templateFileInfo, 0, 10),
	}
	for r := range cmd.filesCh {
		p, h := r[0], r[1]
		rel, _ := filepath.Rel(cmd.srcDir, p)
		rel = filepath.ToSlash(rel)

		key := rel
		name := key
		url := cmd.urlPrefix + "/" + name
		if h != "" {
			if matched, _ := filepath.Match("*.woff2", name); matched {
				url += "?v=" + h
			} else if cmd.server == "gin" && !cmd.isEmbed {
			} else {
				ext := filepath.Ext(name)
				name = name[:len(name)-len(ext)] + "." + h + ext
				url = cmd.urlPrefix + "/" + name
			}
		}

		model.Files = append(model.Files, &templateFileInfo{
			Key:  key,
			Path: name,
			Url:  url,
		})
	}

	dest, err := os.OpenFile(filepath.Join(cmd.distDir, "assets_gen.go"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer dest.Close()

	buffer := new(bytes.Buffer)
	defer buffer.Reset()
	if err = tmpl.ExecuteTemplate(buffer, "assets.gotmpl", model); err != nil {
		return fmt.Errorf("failed render assets_gen.go: %w", err)
	}
	var b []byte
	if b, err = format.Source(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed format assets_gen.go: %w", err)
	}

	if _, err = io.Copy(dest, bytes.NewReader(b)); err != nil {
		return fmt.Errorf("failed write assets_gen.go: %w", err)
	}
	return nil
}

func (cmd *AssetsHandler) normalizeDirPaths() (err error) {
	if cmd.cwd == "" {
		cmd.cwd, err = os.Getwd()
		if err != nil {
			return
		}
		cmd.cwd = filepath.ToSlash(path.Clean(cmd.cwd))
	}
	cmd.srcDir = filepath.ToSlash(path.Join(cmd.cwd, cmd.srcDir))
	cmd.distDir = filepath.ToSlash(filepath.Join(cmd.cwd, cmd.distDir))
	return
}

func (cmd *AssetsHandler) hashFile(p string) (h string, err error) {
	if !cmd.shouldHash {
		return
	}

	var b []byte
	if b, err = os.ReadFile(p); err != nil {
		return
	} else if len(b) == 0 {
		return "", nil
	}

	h = fmt.Sprintf("%x", crc32.Checksum(b, cmd.hasher))
	return
}
