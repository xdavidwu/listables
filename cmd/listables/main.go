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
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"syscall"

	"github.com/xdavidwu/listables/internal/dirlist"
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

	dirlist.Footer = *foot
	fs := os.DirFS(".")
	staticHandler := http.FileServerFS(fs)
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

			if err := dirlist.Render(w, fs, p); err != nil {
				w.WriteHeader(404)
				return
			}
		} else {
			staticHandler.ServeHTTP(w, r)
		}
	})

	s := http.Server{Handler: h}
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
