package webui

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// publicName is the name of the directory below the base URL, with a
// trailing slash, of the public links, which have the form
// publicName + token + "/" + path
const publicName = "@public/"

// publicPrefix is publicName as a path below the base URL
const publicPrefix = "/" + publicName

// publinksEnabled reports whether public links are made and served
//
// They need the VFS to not depend on the user, so they are off with an auth
// proxy.
func (u *UI) publinksEnabled() bool {
	return len(u.opt.PublinkPaths) > 0 && !u.cfg.IsProxy
}

// allowed reports whether the remote is at or below a path of the options
// for public links
//
// The paths are prefix closed: whatever is below an allowed remote is
// allowed. The path "/" allows everything.
func (u *UI) allowed(remote string) bool {
	for _, entry := range u.opt.PublinkPaths {
		e := strings.Trim(entry, "/")
		if e == "" || remote == e || strings.HasPrefix(remote, e+"/") {
			return true
		}
	}
	return false
}

// pubToken returns the token of the public link to the remote
//
// It doesn't expire and is valid for the remote and everything below it.
func (u *UI) pubToken(remote string) string {
	mac := hmac.New(sha256.New, u.keys.publink)
	_, _ = mac.Write([]byte(remote))
	return hex.EncodeToString(mac.Sum(nil)[:tokenSigLen])
}

// checkPubToken reports whether tok is the token of the remote or of one of
// the directories above it, up to the root
func (u *UI) checkPubToken(tok, remote string) bool {
	for p := remote; ; p = parentRemote(p) {
		if hmac.Equal([]byte(tok), []byte(u.pubToken(p))) {
			return true
		}
		if p == "" {
			return false
		}
	}
}

// parentRemote returns the remote of the directory holding the remote,
// which is "" for the root
func parentRemote(remote string) string {
	if dir := path.Dir(remote); dir != "." {
		return dir
	}
	return ""
}

// publinkToken returns the token to show in a listing for the remote, or
// nil if it has no public link
func (u *UI) publinkToken(remote string) *string {
	if !u.publinksEnabled() || !u.allowed(remote) {
		return nil
	}
	tok := u.pubToken(remote)
	return &tok
}

// publicDownload returns the handler which serves a request for a public
// link, or nil if the request isn't for one and must be left to the next
// handler
//
// Links are served to requests with or without credentials: a browser may
// send the ones it has cached to a link which doesn't need them. Only GET and
// HEAD requests are served, so nothing can be written through a link.
func (u *UI) publicDownload(r *http.Request) http.Handler {
	if !u.publinksEnabled() || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return nil
	}
	rest, ok := strings.CutPrefix(u.relPath(r), publicPrefix)
	if !ok {
		return nil
	}
	return u.publicWrap(u.hold(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The token is in the URL, so keep it from leaking in a Referer
		w.Header().Set("Referrer-Policy", "no-referrer")
		u.servePublic(w, r, rest)
	})))
}

// servePublic replies to a GET or HEAD request for the public link in
// rest, the request path after publicPrefix: a token and a path.
//
// A link which is invalid or for something which doesn't exist is a 404, so
// that the two can't be told apart, and one for something outside the paths
// of the options is a 403.
func (u *UI) servePublic(w http.ResponseWriter, r *http.Request, rest string) {
	notFound := func() { httpError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound)) }
	tok, sub, found := strings.Cut(rest, "/")
	if !found || tok == "" {
		notFound()
		return
	}
	p := "/" + sub
	if u.cfg.CheckPath(p) != nil {
		notFound()
		return
	}
	remote := remoteFromPath(p)
	// The token covers the spelling it is checked for, so the VFS must see
	// the same path
	if remote != "" && path.Clean(remote) != remote {
		notFound()
		return
	}
	if !u.checkPubToken(tok, remote) {
		notFound()
		return
	}
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		httpError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	node, err := VFS.Stat(remote)
	if err != nil {
		fs.Debugf(remote, "webui: public link failed to stat: %v", err)
		notFound()
		return
	}
	if !u.allowed(remote) {
		httpError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		return
	}
	fs.Debugf(remote, "webui: public link access")

	switch n := node.(type) {
	case *vfs.File:
		if strings.HasSuffix(p, "/") {
			notFound()
			return
		}
		u.servePublicFile(w, r, VFS, n, remote)
	case *vfs.Dir:
		u.servePublicDir(w, r, VFS, n, remote, tok, !strings.HasSuffix(p, "/"))
	default:
		notFound()
	}
}

// hasResumeQuery reports whether the query is for the playback resume API
func hasResumeQuery(query url.Values) bool {
	return query.Has("resume-get") || query.Has("resume-save") || query.Has("resume-clear")
}

// servePublicFile replies with the file remote, file, of the VFS: its
// player page, the playback resume API or its content
func (u *UI) servePublicFile(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS, file *vfs.File, remote string) {
	query := r.URL.Query()
	switch {
	case hasResumeQuery(query):
		// This writes only the store of playback positions
		u.serveResumeRemote(w, r, remote)
	case query.Has("view"):
		data := EditData{
			Href:                             "/" + remote,
			Kind:                             kindView,
			URIPrefix:                        u.baseSlash(),
			Editable:                         u.editable(file),
			ResilientPlayerMaxRetry:          u.opt.PlayerMaxRetry,
			ResilientPlayerReconnectInterval: u.opt.PlayerReconnectInterval,
		}
		u.writeData(w, r, remote, &data, false)
	default:
		serveFileRaw(w, r, VFS, remote)
	}
}

// servePublicDir replies for the directory remote, dir, of the VFS reached
// with the token tok: a redirect to the URL with a slash if noSlash is set,
// else a zip of it or its read-only page, or JSON for the page
func (u *UI) servePublicDir(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS, dir *vfs.Dir, remote, tok string, noSlash bool) {
	query := r.URL.Query()
	switch {
	case hasResumeQuery(query):
		// Playback positions are of files
		httpError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	case noSlash && remote != "":
		// The page resolves its links against the current URL so it needs
		// the slash. Built from validated parts so that it can't be an
		// absolute URL.
		loc := url.URL{
			Path:     strings.TrimSuffix(u.baseSlash(), "/") + publicPrefix + tok + "/" + remote + "/",
			RawQuery: r.URL.RawQuery,
		}
		http.Redirect(w, r, loc.String(), http.StatusFound)
		return
	case query.Has("zip"):
		// Everything below an allowed directory is allowed, so the zip holds
		// nothing which isn't.
		u.serveZip(w, r, VFS, remote, query)
		return
	}
	entries, err := dir.ReadDirAll()
	if err != nil {
		vfsError(w, remote, err)
		return
	}
	items := make([]PathItem, 0, len(entries))
	for _, entry := range entries {
		if !u.allowed(path.Join(remote, entry.Name())) {
			continue
		}
		// Neither links nor playback positions are shown to the visitor
		item := u.entryItem(VFS, entry, remote, true)
		item.ResumePosition = nil
		items = append(items, item)
	}
	sortPathItems(items, query.Get("sort"), query.Get("order"), query.Has("sort"))
	data := IndexData{
		Href:         "/" + remote,
		Kind:         "Index",
		URIPrefix:    u.baseSlash() + publicName + tok + "/",
		AllowArchive: !u.cfg.DisableZip,
		DirExists:    true,
		Paths:        items,
	}
	u.writeData(w, r, remote, &data, query.Has("json"))
}
