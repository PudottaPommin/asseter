package asseter

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/valyala/bytebufferpool"
)

type (
	AssetsFsOptions struct {
		Src      string
		Pkg      string
		Out      string
		EmbedDir string
	}
	AssetsFsHandler struct {
		src      string
		pkg      string
		out      string
		embedDir string
	}
	file struct {
		Path             string
		Name             string
		Hash             string
		ModTime          time.Time
		UncompressedSize int
		EmbedPath        string
		IsCompressed     bool
	}
	direntry struct {
		Name    string
		IsDir   bool
		ModTime time.Time
		Size    int64
	}
	assetsFsTemplateModel struct {
		Pkg      string
		EmbedDir string
		Files    []file
		Dirs     map[string][]direntry
	}
)

func NewAssetsFsHandler(o AssetsFsOptions) (*AssetsFsHandler, error) {
	embedDir := o.EmbedDir
	if embedDir == "" {
		embedDir = "_embed"
	}
	embedDir = filepath.ToSlash(filepath.Clean(embedDir))
	if filepath.IsAbs(embedDir) || strings.HasPrefix(embedDir, "../") || embedDir == ".." {
		return nil, fmt.Errorf("embedDir must be a relative subdirectory: %s", o.EmbedDir)
	}

	return &AssetsFsHandler{
		src:      o.Src,
		pkg:      o.Pkg,
		out:      o.Out,
		embedDir: embedDir,
	}, nil
}

func (h *AssetsFsHandler) Run() error {
	root, err := os.OpenRoot(h.src)
	if err != nil {
		return fmt.Errorf("failed to open root: %w", err)
	}
	defer root.Close()
	rfs := root.FS()

	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return err
	}
	defer enc.Close()

	absEmbedDir := filepath.Join(filepath.Dir(h.out), filepath.FromSlash(h.embedDir))
	if err = os.RemoveAll(absEmbedDir); err != nil {
		return fmt.Errorf("failed to remove existing embed dir %s: %w", absEmbedDir, err)
	}
	if err = os.MkdirAll(absEmbedDir, 0o755); err != nil {
		return fmt.Errorf("failed to create embed dir %s: %w", absEmbedDir, err)
	}

	files := make([]file, 0)
	dirs := make(map[string][]direntry)

	var cmpBuf []byte

	if err = fs.WalkDir(rfs, ".", func(fp string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			entries, err := fs.ReadDir(rfs, fp)
			if err != nil {
				return err
			}
			dirEntries := make([]direntry, 0, len(entries))
			for i := range entries {
				fi, _ := entries[i].Info()
				var mt time.Time
				var sz int64
				if fi != nil {
					mt = fi.ModTime()
					sz = fi.Size()
				}
				dirEntries = append(dirEntries, direntry{
					Name:    entries[i].Name(),
					IsDir:   entries[i].IsDir(),
					ModTime: mt,
					Size:    sz,
				})
			}
			dirs[fp] = dirEntries
			return nil
		}

		src, err := fs.ReadFile(rfs, fp)
		if err != nil {
			return err
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		cmpBuf = enc.EncodeAll(src, cmpBuf[:0])

		var (
			embedRelPath string
			isCompressed bool
			dataToWrite  []byte
		)

		if len(cmpBuf) < len(src) {
			embedRelPath = fp + ".zst"
			isCompressed = true
			dataToWrite = make([]byte, len(cmpBuf))
			copy(dataToWrite, cmpBuf)
		} else {
			embedRelPath = fp
			isCompressed = false
			dataToWrite = src
		}

		embedPath := path.Join(h.embedDir, filepath.ToSlash(embedRelPath))
		outFilePath := filepath.Join(absEmbedDir, filepath.FromSlash(embedRelPath))
		if err = os.MkdirAll(filepath.Dir(outFilePath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", outFilePath, err)
		}
		if err = os.WriteFile(outFilePath, dataToWrite, 0o644); err != nil {
			return fmt.Errorf("failed to write embedded asset %s: %w", outFilePath, err)
		}

		files = append(files, file{
			Path:             fp,
			Name:             path.Base(fp),
			Hash:             hashBytes(src),
			ModTime:          fi.ModTime(),
			UncompressedSize: len(src),
			EmbedPath:        embedPath,
			IsCompressed:     isCompressed,
		})
		return nil
	}); err != nil {
		return err
	}

	buffer := bytebufferpool.Get()
	defer bytebufferpool.Put(buffer)
	if err = tmpl.ExecuteTemplate(buffer, "assetsfs.gotmpl", assetsFsTemplateModel{
		Pkg:      h.pkg,
		EmbedDir: h.embedDir,
		Files:    files,
		Dirs:     dirs,
	}); err != nil {
		return fmt.Errorf("failed render outfile.go: %w", err)
	}

	var formatted []byte
	if formatted, err = format.Source(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed format %s: %w", h.out, err)
	}

	out, err := os.OpenFile(h.out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, bytes.NewReader(formatted)); err != nil {
		return fmt.Errorf("failed write %s: %w", h.out, err)
	}

	return nil
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func (f file) HashedName() string {
	if f.Hash == "" {
		return f.Name
	}
	ext := filepath.Ext(f.Path)
	pured := f.Path[:len(f.Path)-len(ext)]
	return fmt.Sprintf("%s.%s%s", pured, f.Hash, ext)
}

var (
	//go:embed layeredfs.gotmpl
	tplFs string
	tmpl  = template.Must(template.New("").Parse(tplFs))
)
