// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// go-aah/aah source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package aah

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"

	"aahframework.org/ahttp.v0"
	"aahframework.org/essentials.v0"
	"aahframework.org/log.v0"
)

// serveStatic method static file/directory delivery.
func (e *engine) serveStatic(ctx *Context) error {
	dir, file := filepath.Split(getFilepath(ctx))
	log.Tracef("Dir: %s, File: %s", dir, file)

	ctx.Reply().gzip = checkGzipRequired(file)
	e.wrapGzipWriter(ctx)
	e.writeHeaders(ctx)

	res, req := ctx.Res, ctx.Req
	fs := ahttp.Dir(dir, ctx.route.ListDir)
	f, err := fs.Open(file)
	if err != nil {
		if err == ahttp.ErrDirListNotAllowed {
			log.Warnf("directory listing not allowed: %s", req.Path)
			res.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(res, "403 Directory listing not allowed")
		} else if os.IsNotExist(err) {
			log.Errorf("file not found: %s", req.Path)
			return errFileNotFound
		} else if os.IsPermission(err) {
			log.Warnf("permission issue: %s", req.Path)
			res.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(res, "403 Forbidden")
		} else {
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, "500 Internal Server Error")
		}

		return nil
	}

	defer ess.CloseQuietly(f)

	fi, err := f.Stat()
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		_, _ = res.Write([]byte("500 Internal Server Error"))
		return nil
	}

	if fi.IsDir() {
		// redirect if the directory name doesn't end in a slash
		if req.Path[len(req.Path)-1] != '/' {
			log.Debugf("redirecting to dir: %s", req.Path+"/")
			http.Redirect(res, req.Raw, path.Base(req.Path)+"/", http.StatusFound)
			return nil
		}

		// 'OnPreReply' server extension point
		publishOnPreReplyEvent(ctx)

		directoryList(res, req.Raw, f)

		// 'OnAfterReply' server extension point
		publishOnAfterReplyEvent(ctx)
		return nil
	}

	// 'OnPreReply' server extension point
	publishOnPreReplyEvent(ctx)

	http.ServeContent(res, req.Raw, file, fi.ModTime(), f)

	// 'OnAfterReply' server extension point
	publishOnAfterReplyEvent(ctx)
	return nil
}

// directoryList method compose directory listing response
func directoryList(res http.ResponseWriter, req *http.Request, f http.File) {
	dirs, err := f.Readdir(-1)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		_, _ = res.Write([]byte("Error reading directory"))
		return
	}
	sort.Sort(byName(dirs))

	res.Header().Set(ahttp.HeaderContentType, ahttp.ContentTypeHTML.Raw())
	reqPath := req.URL.Path
	fmt.Fprintf(res, "<html>\n")
	fmt.Fprintf(res, "<head><title>Listing of %s</title></head>\n", reqPath)
	fmt.Fprintf(res, "<body bgcolor=\"white\">\n")
	fmt.Fprintf(res, "<h1>Listing of %s</h1><hr>\n", reqPath)
	fmt.Fprintf(res, "<pre><table border=\"0\">\n")
	fmt.Fprintf(res, "<tr><td collapse=\"2\"><a href=\"../\">../</a></td></tr>\n")
	for _, d := range dirs {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		// name may contain '?' or '#', which must be escaped to remain
		// part of the URL path, and not indicate the start of a query
		// string or fragment.
		url := url.URL{Path: name}
		fmt.Fprintf(res, "<tr><td><a href=\"%s\">%s</a></td><td width=\"200px\" align=\"right\">%s</td></tr>\n",
			url.String(),
			template.HTMLEscapeString(name),
			d.ModTime().Format(appDefaultDateTimeFormat),
		)
	}
	fmt.Fprintf(res, "</table></pre>\n")
	fmt.Fprintf(res, "<hr></body>\n")
	fmt.Fprintf(res, "</html>\n")
}

// checkGzipRequired method return for static which requires gzip response.
func checkGzipRequired(file string) bool {
	switch filepath.Ext(file) {
	case ".css", ".js", ".html", ".htm", ".json", ".xml",
		".txt", ".csv", ".ttf", ".otf", ".eot":
		return true
	default:
		return false
	}
}

func getFilepath(ctx *Context) string {
	var file string
	if ctx.route.IsDir() {
		file = filepath.Join(AppBaseDir(), ctx.route.Dir, ctx.Req.PathValue("filepath"))
	} else {
		file = filepath.Join(AppBaseDir(), "static", ctx.route.File)
	}
	return filepath.FromSlash(file)
}

// Sort interface for Directory list
func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
