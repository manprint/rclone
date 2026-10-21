package webui

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rclone/rclone/fs"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs"
)

// remoteFromPath returns the path relative to the root of the VFS from a
// URL path with the base URL removed
func remoteFromPath(p string) string {
	return strings.Trim(p, "/")
}

// hasOrigin reports whether the request carries an Origin header, which
// browsers add to the cross-origin and non-GET requests made by scripts
// and WebDAV clients don't.
func hasOrigin(r *http.Request) bool {
	return r.Header.Get("Origin") != ""
}

// lookupUser returns the name of the authenticated user and whether there
// is one
func (u *UI) lookupUser(r *http.Request) (string, bool) {
	if name, ok := libhttp.CtxGetUser(r.Context()); ok {
		return name, true
	}
	// The auth proxy doesn't put the user in the context
	if u.cfg.UsingAuth() {
		if name, _, ok := r.BasicAuth(); ok {
			return name, true
		}
	}
	return "", false
}

// user returns the name of the authenticated user, or "" if there is none
func (u *UI) user(r *http.Request) string {
	name, _ := u.lookupUser(r)
	return name
}

// destRemote returns the remote named by the Destination header of a
// COPY or MOVE request
//
// It returns false if the header doesn't name a path below the base URL on
// this host, or the path is rejected by the server's path check.
func (u *UI) destRemote(r *http.Request) (string, bool) {
	header := r.Header.Get("Destination")
	if header == "" {
		return "", false
	}
	dest, err := url.Parse(header)
	if err != nil {
		return "", false
	}
	if dest.Host != "" && dest.Host != r.Host {
		return "", false
	}
	p := dest.Path
	// The Destination still has the base URL which the router removed
	// from the request path.
	if base := u.baseSlash(); base != "/" {
		rest, ok := strings.CutPrefix(p, base)
		if !ok {
			return "", false
		}
		p = "/" + rest
	}
	if u.cfg.CheckPath(p) != nil {
		return "", false
	}
	return remoteFromPath(p), true
}

// httpError replies with the status and message as plain text
func httpError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

// errStatus returns the HTTP status for an error from the VFS
func errStatus(err error) int {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, vfs.EROFS), errors.Is(err, os.ErrPermission):
		return http.StatusForbidden
	}
	return http.StatusInternalServerError
}

// vfsError replies with the status for an error from the VFS, logging the
// error if it is unexpected
func vfsError(w http.ResponseWriter, remote string, err error) {
	status := errStatus(err)
	if status == http.StatusInternalServerError {
		fs.Errorf(remote, "webui: %v", err)
	}
	httpError(w, status, http.StatusText(status))
}
