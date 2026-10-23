package webui

import (
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// serveFileRaw replies to a GET or HEAD request with the file remote of
// the VFS
//
// Range, conditional and HEAD requests are handled. A remote which isn't a
// file is a 404. The files which a browser would run scripts from are
// sandboxed.
func (u *UI) serveFileRaw(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS, remote string) {
	node, err := u.lookups.stat(r.Context(), VFS, remote)
	if err != nil {
		vfsError(w, remote, err)
		return
	}
	file, ok := node.(*vfs.File)
	if !ok {
		httpError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}
	h, err := file.Open(os.O_RDONLY)
	if err != nil {
		vfsError(w, remote, err)
		return
	}
	defer func() {
		if err := h.Close(); err != nil {
			fs.Debugf(remote, "webui: failed to close file: %v", err)
		}
	}()
	if activeExts[strings.ToLower(path.Ext(remote))] {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	var modTime time.Time
	if !VFS.Opt.NoModTime {
		modTime = file.ModTime()
	}
	http.ServeContent(w, r, path.Base(remote), modTime, h)
}
