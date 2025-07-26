package asseter

import (
	"flag"
	"path/filepath"
	"strings"
)

type FileMatchFlag []string

var _ flag.Value = (*FileMatchFlag)(nil)

func (e *FileMatchFlag) String() string {
	return strings.Join(*e, ",")
}

func (e *FileMatchFlag) Set(value string) error {
	*e = append(*e, value)
	return nil
}

func (e *FileMatchFlag) Match(path string) bool {
	for _, pattern := range *e {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

type FontSourceFontsFlag []string

var _ flag.Value = (*FontSourceFontsFlag)(nil)

func (e *FontSourceFontsFlag) String() string {
	return strings.Join(*e, ",")
}

func (e *FontSourceFontsFlag) Set(value string) error {
	*e = append(*e, value)
	return nil
}

func (e *FontSourceFontsFlag) Match(path string) bool {
	for _, s := range *e {
		if matched, _ := filepath.Match(s+"*", path); matched {
			return true
		}
	}
	return false
}
