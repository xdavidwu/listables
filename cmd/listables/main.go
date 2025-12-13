package main

// #cgo LDFLAGS: -static
// #define _GNU_SOURCE
// #include <sched.h>
// __attribute__((constructor)) void f() {
//	unshare(CLONE_NEWUSER);
// }
import "C"

import (
	"flag"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/xdavidwu/listables/internal/template"
)

var (
	addr = flag.String("l", "0.0.0.0:8000", "Listen on address")
	root = flag.String("r", ".", "Root of content")
	foot = flag.String("f", "", "Footer message")
)

func main() {
	flag.Parse()

	// load mime map before chroot'ing
	mime.TypeByExtension(".o")

	if err := os.Chdir(*root); err != nil {
		panic(err)
	}
	if err := syscall.Chroot(*root); err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		panic(err)
	}
	f, ok := os.DirFS(".").(fs.ReadDirFS)
	if !ok {
		panic("fs impl not supporting fs.ReadDirFS")
	}
	sf, ok := f.(fs.StatFS)
	if !ok {
		panic("fs impl not supporting fs.StatFS")
	}

	staticHandler := http.FileServerFS(f)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := len(r.URL.Path)
		if r.URL.Path[l-1] == '/' {
			p := r.URL.Path
			if r.URL.Path[0] != '/' {
				p = "/" + r.URL.Path
			}
			p = path.Clean(p)
			// net/http.ioFS
			if p == "/" {
				p = "."
			} else {
				p = p[1:]
			}
			ds, err := f.ReadDir(p)
			if err != nil {
				w.WriteHeader(404)
				return
			}

			entries := map[string]fs.FileInfo{}
			for _, d := range ds {
				dname := d.Name()
				fp := path.Clean(p + "/" + dname)
				if d.Type()&fs.ModeSymlink == fs.ModeSymlink {
					e, err := sf.Stat(fp)
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

			template.Template.Execute(w, template.Data{
				Path:    r.URL.Path,
				Entries: entries,
				Footer:  *foot,
			})
		} else {
			staticHandler.ServeHTTP(w, r)
		}
	})

	s := http.Server{Handler: h}
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
