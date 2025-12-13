package dirlist

import (
	"io"
	"io/fs"
	"log/slog"
	"path"
)

var (
	Footer = ""
)

func Render(w io.Writer, f fs.FS, p string) error {
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
		return nil
	}

	entries := map[string]fs.FileInfo{}
	for _, d := range ds {
		dname := d.Name()
		fp := path.Join(p, dname)
		if d.Type()&fs.ModeSymlink == fs.ModeSymlink {
			e, err := sfs.Stat(fp)
			if err != nil {
				slog.Warn("cannot stat symlink", "file", fp, "error", err)
			} else {
				entries[dname] = e
			}
		} else {
			e, err := d.Info()
			if err != nil {
				slog.Warn("cannot fs.DirEntry.Info()", "file", fp, "error", err)
			} else {
				entries[dname] = e
			}
		}
	}

	dp := p
	if p == "" || p == "." {
		dp = "/"
	} else if p[0] != '/' {
		dp = "/" + p
	}
	return Template.Execute(w, Data{
		Path:    dp,
		Entries: entries,
		Footer:  Footer,
	})
}
