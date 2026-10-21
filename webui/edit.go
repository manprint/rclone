package webui

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// sniffLen is the number of bytes at the start of a file which decide
// whether it is text
const sniffLen = 1024

// Kinds of the pages for a file
const (
	kindEdit = "Edit"
	kindView = "View"
)

// activeExts are the lowercased extensions of the files which a browser
// runs scripts from when it opens them
var activeExts = map[string]bool{
	".html":  true,
	".htm":   true,
	".xhtml": true,
	".svg":   true,
	".svgz":  true,
	".xml":   true,
	".xsl":   true,
}

// isText reports whether the first bytes of a file look like text: they
// hold no NUL and are valid UTF-8.
//
// A full buffer was cut at an arbitrary byte so an incomplete character at
// its end is ignored.
func isText(b []byte) bool {
	if bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	if len(b) >= sniffLen {
		b = trimPartialRune(b)
	}
	return utf8.Valid(b)
}

// trimPartialRune returns b without a truncated UTF-8 sequence at its end
func trimPartialRune(b []byte) []byte {
	for i := 1; i < utf8.UTFMax && i <= len(b); i++ {
		tail := b[len(b)-i:]
		if utf8.RuneStart(tail[0]) {
			if !utf8.FullRune(tail) {
				return b[:len(b)-i]
			}
			break
		}
	}
	return b
}

// editable reports whether the file is small enough, and looks enough
// like text, to be edited. A file which can't be read isn't editable.
func (u *UI) editable(file *vfs.File) bool {
	if file.Size() > int64(u.opt.EditMaxSize) {
		return false
	}
	h, err := file.Open(os.O_RDONLY)
	if err != nil {
		fs.Debugf(file.Path(), "webui: failed to open file to check it is text: %v", err)
		return false
	}
	defer func() { _ = h.Close() }()
	prefix, err := io.ReadAll(io.LimitReader(h, sniffLen))
	if err != nil {
		fs.Debugf(file.Path(), "webui: failed to read file to check it is text: %v", err)
		return false
	}
	return isText(prefix)
}

// serveEdit handles GET and HEAD requests for the edit and view pages of a
// file. It returns false if the request isn't one for them and must be
// handled by the next handler.
func (u *UI) serveEdit(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	var kind string
	switch query := r.URL.Query(); {
	case query.Has("edit"):
		kind = kindEdit
	case query.Has("view"):
		kind = kindView
	default:
		return false
	}
	remote := remoteFromPath(r.URL.Path)
	if remote == "" || strings.HasSuffix(r.URL.Path, "/") {
		return false
	}
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		return false
	}
	node, err := VFS.Stat(remote)
	if err != nil {
		return false
	}
	file, ok := node.(*vfs.File)
	if !ok {
		return false
	}

	var user *string
	if name, ok := u.lookupUser(r); ok {
		user = &name
	}
	data := EditData{
		Href:                             "/" + remote,
		Kind:                             kind,
		URIPrefix:                        u.baseSlash(),
		AllowUpload:                      !VFS.Opt.ReadOnly,
		AllowDelete:                      !VFS.Opt.ReadOnly,
		Auth:                             u.cfg.UsingAuth(),
		User:                             user,
		Editable:                         u.editable(file),
		ResilientPlayerMaxRetry:          u.opt.PlayerMaxRetry,
		ResilientPlayerReconnectInterval: u.opt.PlayerReconnectInterval,
	}
	u.writeData(w, r, remote, &data, false)
	return true
}

// sandboxActive sets the sandbox policy on a GET or HEAD request for a
// file which a browser would run scripts from when opening it, so that
// they can't reach the server with the user's credentials
func sandboxActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") || !activeExts[strings.ToLower(path.Ext(r.URL.Path))] {
		return
	}
	w.Header().Set("Content-Security-Policy", "sandbox")
}
