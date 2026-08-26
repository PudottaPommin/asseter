package asseter_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pudottapommin/asseter"
)

func TestAssetsFsGenerator(t *testing.T) {
	tempDir := t.TempDir()

	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(filepath.Join(srcDir, "css"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "images"), 0755); err != nil {
		t.Fatal(err)
	}

	compressibleContent := []byte(strings.Repeat("body { background-color: #ffffff; color: #000000; }\n", 100))
	if err := os.WriteFile(filepath.Join(srcDir, "css", "style.css"), compressibleContent, 0644); err != nil {
		t.Fatal(err)
	}

	tinyContent := []byte("hi")
	if err := os.WriteFile(filepath.Join(srcDir, "images", "tiny.txt"), tinyContent, 0644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tempDir, "assets")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(outDir, "bindata.asseter.go")

	handler, err := asseter.NewAssetsFsHandler(asseter.AssetsFsOptions{
		Src:      srcDir,
		Pkg:      "assets",
		Out:      outFile,
		EmbedDir: "_embed",
	})
	if err != nil {
		t.Fatalf("NewAssetsFsHandler failed: %v", err)
	}

	if err := handler.Run(); err != nil {
		t.Fatalf("handler.Run failed: %v", err)
	}

	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("Expected output file %s to exist: %v", outFile, err)
	}

	embedDir := filepath.Join(outDir, "_embed")
	if _, err := os.Stat(embedDir); err != nil {
		t.Fatalf("Expected embed directory %s to exist: %v", embedDir, err)
	}

	cssZst := filepath.Join(embedDir, "css", "style.css.zst")
	if _, err := os.Stat(cssZst); err != nil {
		t.Fatalf("Expected compressed file %s to exist: %v", cssZst, err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	goCode := string(content)

	if !strings.Contains(goCode, "//go:embed all:_embed") {
		t.Errorf("Expected go file to contain `//go:embed all:_embed`, got:\n%s", goCode)
	}
	if !strings.Contains(goCode, `"_embed/css/style.css.zst"`) {
		t.Errorf("Expected go file to contain embed path for style.css, got:\n%s", goCode)
	}

	mainGoContent := fmt.Sprintf(`package main

import (
	"fmt"
	"io"
	"os"

	"testmod/assets"
)

type zstdProvider interface {
	ZstdBytes() []byte
}

func main() {
	f, err := assets.Assets.Open("css/style.css")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open style.css: %%v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read style.css: %%v\n", err)
		os.Exit(1)
	}

	expectedLen := %d
	if len(data) != expectedLen {
		fmt.Fprintf(os.Stderr, "expected len %%d, got %%d\n", expectedLen, len(data))
		os.Exit(1)
	}

	// Test Seeking on compressed file
	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		fmt.Fprintf(os.Stderr, "f does not implement io.ReadSeeker\n")
		os.Exit(1)
	}
	pos, err := seeker.Seek(0, io.SeekStart)
	if err != nil || pos != 0 {
		fmt.Fprintf(os.Stderr, "seek start failed: pos=%%d, err=%%v\n", pos, err)
		os.Exit(1)
	}
	buf := make([]byte, 4)
	if n, err := f.Read(buf); err != nil || n != 4 || string(buf) != "body" {
		fmt.Fprintf(os.Stderr, "read after seek failed: n=%%d, err=%%v, buf=%%s\n", n, err, string(buf))
		os.Exit(1)
	}
	pos, err = seeker.Seek(2, io.SeekCurrent)
	if err != nil || pos != 6 {
		fmt.Fprintf(os.Stderr, "seek current failed: pos=%%d, err=%%v\n", pos, err)
		os.Exit(1)
	}

	// Test ZstdBytes
	zp, ok := f.(zstdProvider)
	if !ok || len(zp.ZstdBytes()) == 0 {
		fmt.Fprintf(os.Stderr, "ZstdBytes() failed or empty\n")
		os.Exit(1)
	}

	// Test Hash and HashedName on file
	type fileDetails interface {
		Hash() string
		HashedName() string
	}
	fd, ok := f.(fileDetails)
	if !ok {
		fmt.Fprintf(os.Stderr, "f does not implement fileDetails\n")
		os.Exit(1)
	}
	if len(fd.Hash()) != 16 {
		fmt.Fprintf(os.Stderr, "expected 16-char hash, got %%d chars: %%s\n", len(fd.Hash()), fd.Hash())
		os.Exit(1)
	}

	hashed := assets.Assets.HashedByPath("css/style.css")
	if hashed == "" || hashed == "css/style.css" {
		fmt.Fprintf(os.Stderr, "expected valid hashed name, got %%s\n", hashed)
		os.Exit(1)
	}
	if hashed != fd.HashedName() {
		fmt.Fprintf(os.Stderr, "expected hashed name %%s, got %%s\n", fd.HashedName(), hashed)
		os.Exit(1)
	}

	hf, err := assets.Assets.Open(hashed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open by hashed name %%s: %%v\n", hashed, err)
		os.Exit(1)
	}
	defer hf.Close()

	entries, err := assets.Assets.ReadDir("css")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to ReadDir css: %%v\n", err)
		os.Exit(1)
	}
	if len(entries) != 1 || entries[0].Name() != "style.css" {
		fmt.Fprintf(os.Stderr, "unexpected entries: %%+v\n", entries)
		os.Exit(1)
	}

	// Test uncompressed / tiny file
	tf, err := assets.Assets.Open("images/tiny.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open tiny.txt: %%v\n", err)
		os.Exit(1)
	}
	defer tf.Close()

	tdata, err := io.ReadAll(tf)
	if err != nil || string(tdata) != "hi" {
		fmt.Fprintf(os.Stderr, "unexpected tiny.txt content: %%s, err: %%v\n", string(tdata), err)
		os.Exit(1)
	}

	// Test concurrent access (pool safety)
	const goroutines = 20
	errCh := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			cf, err := assets.Assets.Open("css/style.css")
			if err != nil {
				errCh <- err
				return
			}
			defer cf.Close()
			cdata, err := io.ReadAll(cf)
			if err != nil {
				errCh <- err
				return
			}
			if len(cdata) != expectedLen {
				errCh <- fmt.Errorf("unexpected length: %%d", len(cdata))
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < goroutines; i++ {
		if err := <-errCh; err != nil {
			fmt.Fprintf(os.Stderr, "concurrency error: %%v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("OK")
}
`, len(compressibleContent))

	modDir := tempDir
	goModContent := `module testmod

go 1.24

require (
	github.com/klauspost/compress v1.18.6
)
`
	if err := os.WriteFile(filepath.Join(modDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "main.go"), []byte(mainGoContent), 0644); err != nil {
		t.Fatal(err)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = modDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	cmd := exec.Command("go", "run", "-tags", "bindata", "main.go")
	cmd.Dir = modDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run test program with generated assets:\nerr: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	if strings.TrimSpace(stdout.String()) != "OK" {
		t.Fatalf("Unexpected output from test program: %s", stdout.String())
	}

	benchTestContent := `package main

import (
	"io"
	"testing"

	"testmod/assets"
)

func BenchmarkOpenCompressed(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, err := assets.Assets.Open("css/style.css")
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, f)
		_ = f.Close()
	}
}

func BenchmarkOpenUncompressed(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f, err := assets.Assets.Open("images/tiny.txt")
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, f)
		_ = f.Close()
	}
}
`
	if err := os.WriteFile(filepath.Join(modDir, "assets_bench_test.go"), []byte(benchTestContent), 0644); err != nil {
		t.Fatal(err)
	}

	benchCmd := exec.Command("go", "test", "-tags", "bindata", "-bench", ".", "-benchmem", "assets_bench_test.go")
	benchCmd.Dir = modDir
	var benchOut bytes.Buffer
	benchCmd.Stdout = &benchOut
	benchCmd.Stderr = &benchOut
	if err := benchCmd.Run(); err != nil {
		t.Fatalf("Benchmark failed: %v\n%s", err, benchOut.String())
	}
	t.Logf("Benchmark results:\n%s", benchOut.String())
}

func TestAssetsFsGeneratorCustomEmbedDir(t *testing.T) {
	tempDir := t.TempDir()

	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tempDir, "pkg")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(outDir, "bindata.go")

	handler, err := asseter.NewAssetsFsHandler(asseter.AssetsFsOptions{
		Src:      srcDir,
		Pkg:      "pkg",
		Out:      outFile,
		EmbedDir: "_custom_assets",
	})
	if err != nil {
		t.Fatalf("NewAssetsFsHandler failed: %v", err)
	}

	if err := handler.Run(); err != nil {
		t.Fatalf("handler.Run failed: %v", err)
	}

	customDir := filepath.Join(outDir, "_custom_assets")
	if _, err := os.Stat(customDir); err != nil {
		t.Fatalf("Expected custom embed dir %s to exist: %v", customDir, err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "//go:embed all:_custom_assets") {
		t.Errorf("Expected //go:embed all:_custom_assets in generated code")
	}
	if !strings.Contains(string(content), `"_custom_assets/hello.txt`) {
		t.Errorf("Expected _custom_assets/hello.txt in generated code")
	}
}

