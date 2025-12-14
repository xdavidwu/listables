package main

import (
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"slices"

	"github.com/xdavidwu/listables/internal/dirlist"
)

var (
	root = flag.String("r", ".", "Root of content")
	foot = flag.String("f", "", "Footer message")
)

const (
	indexFilename = "index.html"
)

func render(f fs.FS, p string) error {
	rdfs, ok := f.(fs.ReadDirFS)
	if !ok {
		panic("fs impl not supporting fs.ReadDirFS")
	}
	sfs, ok := f.(fs.StatFS)
	if !ok {
		panic("fs impl not supporting fs.StatFS")
	}

	ds, err := rdfs.ReadDir(p)
	if err != nil {
		slog.Error("cannot readdir", "path", p, "error", err)
		return err
	}

	ds = slices.DeleteFunc(ds, func(d fs.DirEntry) bool {
		return d.Name() == indexFilename
	})

	// XXX io/fs.FS is not writable yet
	indexFile, err := os.Create(path.Join(*root, p, indexFilename))
	if err != nil {
		slog.Error("cannot create index file", "path", p, "error", err)
		return err
	}
	defer indexFile.Close()

	if err := dirlist.Template.Execute(indexFile, dirlist.Data{
		Path:    dirlist.UrlPath(p),
		Entries: dirlist.Collect(ds, sfs, p),
		Footer:  *foot,
	}); err != nil {
		slog.Error("cannot write index", "path", p, "error", err)
		return err
	}

	for _, d := range ds {
		if d.IsDir() {
			render(f, path.Join(p, d.Name()))
		}
	}
	return nil
}

func main() {
	flag.Parse()

	dirlist.Footer = *foot
	fs := os.DirFS(*root)
	render(fs, ".")
}
