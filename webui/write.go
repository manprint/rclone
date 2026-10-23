package webui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// updateRangeHeader is the header with which the UI asks for a PATCH to
// append to a file
const updateRangeHeader = "X-Update-Range"

// authenticate replies 401 asking the client to log in
func (u *UI) authenticate(w http.ResponseWriter) {
	realm := u.cfg.Realm
	if realm == "" {
		realm = "rclone"
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
	w.WriteHeader(http.StatusUnauthorized)
}

// serveCheckAuth handles the CHECKAUTH request with which the UI finds
// out who is logged in. It replies with the user name, or if the query has
// "login" and there is no user, asks the browser to log in.
func (u *UI) serveCheckAuth(w http.ResponseWriter, r *http.Request) {
	user := u.user(r)
	if user == "" && r.URL.Query().Has("login") {
		u.authenticate(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, user)
}

// serveLogout handles the LOGOUT request. Replying 401 makes the browser
// drop the credentials it holds for the realm.
func (u *UI) serveLogout(w http.ResponseWriter, _ *http.Request) {
	u.authenticate(w)
}

// bodyReader is a request body which records the error of reading it
type bodyReader struct {
	io.Reader
	err error
}

func (b *bodyReader) Read(p []byte) (n int, err error) {
	n, err = b.Reader.Read(p)
	if err != nil && err != io.EOF {
		b.err = err
	}
	return n, err
}

// copyBody copies the request body to the file and closes it, reporting
// whether all of the body was written and, if it wasn't, whether the
// client rather than the file was at fault
func copyBody(h vfs.Handle, r *http.Request) (complete, byClient bool, err error) {
	body := &bodyReader{Reader: r.Body}
	n, cerr := io.Copy(h, body)
	clerr := h.Close()
	err = errors.Join(cerr, clerr)
	// A short body without an error happens if the connection is closed
	// at the end of a message.
	short := r.ContentLength >= 0 && n != r.ContentLength
	if body.err != nil {
		return false, true, body.err
	}
	return err == nil && !short, err == nil, err
}

// uploadIncomplete replies to an upload which wasn't written in full,
// with 400 if the client was at fault and the status of err if the file was
func uploadIncomplete(w http.ResponseWriter, remote string, byClient bool, err error) {
	if byClient {
		httpError(w, http.StatusBadRequest, "Upload incomplete")
		return
	}
	vfsError(w, remote, err)
}

// servePut handles a PUT from a browser, creating the parent directories
// as needed. It returns false if the request must be handled by the next
// handler.
func (u *UI) servePut(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS) bool {
	remote := remoteFromPath(r.URL.Path)
	if remote == "" || strings.HasSuffix(r.URL.Path, "/") || VFS.Opt.ReadOnly {
		return false
	}
	if node, err := VFS.Stat(remote); err == nil && node.IsDir() {
		return false
	}
	if parent := path.Dir(remote); parent != "." {
		if err := VFS.MkdirAll(parent, 0777); err != nil {
			vfsError(w, remote, err)
			return true
		}
	}
	h, err := VFS.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		vfsError(w, remote, err)
		return true
	}
	complete, byClient, err := copyBody(h, r)
	if !complete {
		fs.Infof(remote, "webui: upload incomplete: %v", err)
		// Without the cache the bytes received are already on the
		// remote and nothing can append to them later. With it the
		// partial file is kept so that the UI can resume the upload
		// with a PATCH.
		if VFS.Opt.CacheMode < vfscommon.CacheModeWrites {
			if rerr := VFS.Remove(remote); rerr != nil && !errors.Is(rerr, vfs.ENOENT) {
				fs.Errorf(remote, "webui: failed to remove incomplete upload: %v", rerr)
			}
		}
		uploadIncomplete(w, remote, byClient, err)
		return true
	}
	w.WriteHeader(http.StatusCreated)
	return true
}

// servePatch handles a PATCH from a browser appending the body to a file.
// It returns false if the request must be handled by the next handler.
func (u *UI) servePatch(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS) bool {
	if r.Header.Get(updateRangeHeader) != "append" {
		httpError(w, http.StatusBadRequest, "Invalid X-Update-Range Header")
		return true
	}
	remote := remoteFromPath(r.URL.Path)
	if VFS.Opt.ReadOnly {
		httpError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		return true
	}
	node, err := VFS.Stat(remote)
	if err != nil {
		vfsError(w, remote, err)
		return true
	}
	if node.IsDir() {
		httpError(w, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
		return true
	}
	// Without the cache the VFS can't append to an existing file.
	if VFS.Opt.CacheMode < vfscommon.CacheModeWrites {
		httpError(w, http.StatusMethodNotAllowed, "Append requires --vfs-cache-mode writes or full")
		return true
	}
	h, err := VFS.OpenFile(remote, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		vfsError(w, remote, err)
		return true
	}
	if complete, byClient, err := copyBody(h, r); !complete {
		// The bytes received are kept so that the upload can go on
		fs.Infof(remote, "webui: append incomplete: %v", err)
		uploadIncomplete(w, remote, byClient, err)
		return true
	}
	w.WriteHeader(http.StatusNoContent)
	return true
}

// guardMove refuses a MOVE from a browser which would replace a directory
//
// The UI doesn't ask for confirmation when moving onto a directory and
// sends no Overwrite header, but the server then sets it to T which
// deletes the destination and everything in it. It returns false if the
// request must be handled by the next handler.
func (u *UI) guardMove(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS) bool {
	if r.Header.Get("Overwrite") != "" {
		return false
	}
	dst, ok := u.destRemote(r)
	if !ok {
		return false
	}
	node, err := u.lookups.stat(r.Context(), VFS, dst)
	switch {
	case errors.Is(err, vfs.ENOENT):
		return false
	case err != nil:
		// The destination may be a directory which the move would delete
		vfsError(w, dst, err)
		return true
	case node.IsDir():
		httpError(w, http.StatusPreconditionFailed, "Destination folder exists")
		return true
	}
	return false
}

// statRemote looks remote up in the VFS and, if the VFS doesn't know it, on
// the backend, as the directory cache may predate a file added by another
// client. found is false if remote doesn't exist, and err is set if it
// can't be told whether it does.
func (u *UI) statRemote(ctx context.Context, VFS *vfs.VFS, remote string) (isDir bool, size int64, found bool, err error) {
	node, err := u.lookups.stat(ctx, VFS, remote)
	if err == nil {
		return node.IsDir(), node.Size(), true, nil
	}
	if !errors.Is(err, vfs.ENOENT) {
		return false, 0, false, err
	}
	o, err := VFS.Fs().NewObject(ctx, remote)
	switch {
	case err == nil:
		return false, o.Size(), true, nil
	case errors.Is(err, fs.ErrorIsDir):
		return true, 0, true, nil
	case errors.Is(err, fs.ErrorObjectNotFound), errors.Is(err, fs.ErrorDirNotFound):
		// A missing parent directory is reported as not found by some
		// backends and by combine outside its upstreams
		return false, 0, false, nil
	}
	return false, 0, false, err
}

// refuseDelete replies 403, when --webui-allow-delete is off, to the
// requests of any client which would delete or replace existing contents:
// DELETE, MOVE (which deletes the source), a PUT onto a file which isn't
// empty and a COPY onto an existing destination. It reports whether it
// replied.
//
// A PUT onto an empty file is let through, as superdufs does, so that a
// file made by the UI can be written once.
func (u *UI) refuseDelete(w http.ResponseWriter, r *http.Request) bool {
	if u.opt.AllowDelete {
		return false
	}
	var remote string
	switch r.Method {
	case http.MethodDelete, "MOVE":
		u.forbid(w, r)
		return true
	case http.MethodPut:
		if u.cfg.CheckPath(r.URL.Path) != nil || strings.HasSuffix(r.URL.Path, "/") {
			return false
		}
		remote = remoteFromPath(r.URL.Path)
	case "COPY":
		// Without overwriting the server refuses an existing destination
		if strings.EqualFold(r.Header.Get("Overwrite"), "F") {
			return false
		}
		dst, ok := u.destRemote(r)
		if !ok {
			return false
		}
		remote = dst
	default:
		return false
	}
	// What can't be checked is refused rather than let through
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		vfsError(w, remote, err)
		return true
	}
	isDir, size, found, err := u.statRemote(r.Context(), VFS, remote)
	if err != nil {
		vfsError(w, remote, err)
		return true
	}
	if !found || (r.Method == http.MethodPut && (isDir || size == 0)) {
		return false
	}
	u.forbid(w, r)
	return true
}

// forbid replies 403 to a request refused by --webui-allow-delete
func (u *UI) forbid(w http.ResponseWriter, r *http.Request) {
	fs.Debugf(nil, "webui: %s %q refused by --webui-allow-delete=false", r.Method, r.URL.Path)
	httpError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
}

// serveWrite handles the requests which change the contents of the
// server. It returns false if the request must be handled by the next
// handler.
//
// PUT, PATCH and MOVE are only taken from browsers, which send an Origin
// header, so that the behaviour for other WebDAV clients is not changed.
func (u *UI) serveWrite(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case "CHECKAUTH":
		u.serveCheckAuth(w, r)
		return true
	case "LOGOUT":
		u.serveLogout(w, r)
		return true
	}
	if !hasOrigin(r) {
		return false
	}
	var handle func(http.ResponseWriter, *http.Request, *vfs.VFS) bool
	switch r.Method {
	case http.MethodPut:
		handle = u.servePut
	case http.MethodPatch:
		if _, ok := r.Header[http.CanonicalHeaderKey(updateRangeHeader)]; !ok {
			return false
		}
		handle = u.servePatch
	case "MOVE":
		handle = u.guardMove
	default:
		return false
	}
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		return false
	}
	return handle(w, r, VFS)
}
