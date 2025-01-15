package main

// #cgo LDFLAGS: -static
// #define _GNU_SOURCE
// #include <sched.h>
// __attribute__((constructor)) void f() {
//	unshare(CLONE_NEWUSER);
// }
import "C"

import (
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"
)

var (
	numfmtSuffix = []string{"", "K", "M", "G", "T"}

	tpl = template.Must(template.New("dirlisting").Funcs(template.FuncMap{
		"timefmt": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"numfmt": func(i int64) string {
			f := float64(i)
			idx := 0
			for f > 1000 && idx < len(numfmtSuffix)-1 {
				f /= 1024
				idx += 1
			}
			if idx == 0 {
				return strconv.FormatInt(i, 10)+" " // for better alignment
			}
			return strconv.FormatFloat(f, 'f', 1, 64) + numfmtSuffix[idx]
		},
	}).Parse(`<!doctype html>
<html>
<head>
	<meta name="viewport" content="initial-scale=1">
	<style>
		body {
			background-color: #fafafa;
			padding: 8px;
		}
		h1 {
			position: sticky;
			font-family: sans-serif;
			line-height: 18px;
			top: 0px;
			margin: -16px -16px 16px -16px;
			padding: 20px;
			padding-left: 32px;
			color: white;
			background-color: #3f51b5;
			box-shadow: 0 2px 4px rgba(0,0,0,.5);
			font-size: 18px;
			letter-spacing: 1px;
		}
		table {
			margin: 4px;
			font-size: 16px;
			letter-spacing: 0.5px;
		}
		th, td {
			font-family: monospace;
			text-align: left;
			line-height: 24px;
			padding-right: 16px;
			white-space: pre;
		}
		th {
			font-weight: normal;
			padding-bottom: 4px;
		}
		th:nth-child(3), td:nth-child(3) {
			text-align: right;
		}
		div {
			border-radius: 2px;
			background-color: white;
			box-shadow: 0 1px 1px 0 rgba(60,64,67,.08),0 1px 3px 1px rgba(60,64,67,.16);
			padding: 16px;
			overflow: auto;
		}
		a {
			text-decoration: none;
		}
		@media (prefers-color-scheme: dark) {
			body {
				background: black;
				color: white;
			}
			div {
				background: #202124;
			}
			a, a:active {
				color: #9e9eff;
			}
			a:visited {
				color: #d0adf0;
			}
			a:active, a:visited:active {
				color: #ff9e9e;
			}
		}
	</style>
	<script>
		const sortBy = (idx) => {
			const els = Array.from(document.querySelectorAll('tbody>tr'));
			const dotdot = els.shift();

			const textCmp = (a, b) =>
				a.children[idx].innerText.localeCompare(b.children[idx].innerText);
			const numCmp = (a, b) =>
				parseInt(a.children[idx].dataset.numeric, 10) - parseInt(b.children[idx].dataset.numeric, 10);
			els.sort(idx == 2 ? numCmp : textCmp);

			els.splice(0, 0, dotdot);
			document.querySelector('tbody').replaceChildren(...els);
		};
	</script>
	<title>Index of {{.Path}}</title>
</head>
<body>
	<h1>Index of {{.Path}}</h1>
	<div><table>
		<thead>
			<tr>
				<th><a onclick="sortBy(0)">Name</a></th>
				<th><a onclick="sortBy(1)">Last Modified</a></th>
				<th><a onclick="sortBy(2)">Size</a></th>
			</tr>
		</thead>
		<tbody>
			<tr><td><a href="..">..</a></td></tr>
		{{range .Entries}}
			<tr>
				<td><a href="{{.Name}}">{{.Name}}{{if .IsDir}}/{{end}}</a></td>
				{{with .Info}}
					<td>{{.ModTime | timefmt}}</td>
					{{if .IsDir}}
						<td data-numeric="0">- </td>
					{{else}}
						<td data-numeric="{{.Size}}">{{.Size | numfmt}}</td>
					{{end}}
				{{end}}
			</tr>
		{{end}}
		</tbody>
	</div></table>
</body>
</html>
`))
)

type Data struct {
	Path    string
	Entries []fs.DirEntry
}

func main() {
	if err := syscall.Chroot("."); err != nil {
		panic(err)
	}

	l, err := net.Listen("tcp", "0.0.0.0:8000")
	if err != nil {
		panic(err)
	}
	f, ok := os.DirFS(".").(fs.ReadDirFS)
	if !ok {
		panic("fs impl not supporting fs.ReadDirFS")
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
			tpl.Execute(w, Data{r.URL.Path, ds})
		} else {
			staticHandler.ServeHTTP(w, r)
		}
	})

	s := http.Server{Handler: h}
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
