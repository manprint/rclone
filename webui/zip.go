package webui

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/vfs"
)

// maxSelectionPayload is the maximum length of the paths parameter of a
// selection zip
const maxSelectionPayload = 8192

// parseSelection parses the paths parameter of a selection zip, a JSON
// array of paths relative to the directory being zipped
//
// It returns the cleaned paths without slashes at their ends, repeats or
// empty entries, with at most max of them. If the parameter is rejected
// it returns the HTTP status and message to reply with instead.
func parseSelection(raw string, max int) (rels []string, status int, msg string) {
	if raw == "" {
		return nil, http.StatusBadRequest, "Missing paths parameter"
	}
	if len(raw) > maxSelectionPayload {
		return nil, http.StatusBadRequest, "Selection payload too large"
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, http.StatusBadRequest, "Invalid selection payload"
	}
	if len(paths) == 0 {
		return nil, http.StatusBadRequest, "Selection is empty"
	}
	if len(paths) > max {
		return nil, http.StatusBadRequest, "Too many selected paths"
	}
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		p = strings.Trim(p, "/")
		if p == "" {
			continue
		}
		if strings.Contains(p, `\`) {
			return nil, http.StatusBadRequest, "Invalid selection path"
		}
		for _, segment := range strings.Split(p, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return nil, http.StatusBadRequest, "Invalid selection path"
			}
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		rels = append(rels, p)
	}
	if len(rels) == 0 {
		return nil, http.StatusBadRequest, "Selection is empty"
	}
	return rels, http.StatusOK, ""
}

// serveZip replies to a GET or HEAD request with a zip of the directory
// dirRemote of the VFS
//
// The query says what to zip: with zip=multi the directory entries named by
// its paths parameter (see parseSelection) and otherwise the whole
// directory. The zip is streamed so an error once the body has started is
// only logged.
func (u *UI) serveZip(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS, dirRemote string, query url.Values) {
	ctx := r.Context()
	if u.cfg.DisableZip {
		httpError(w, http.StatusForbidden, "Zip downloads are disabled")
		return
	}
	node, err := VFS.Stat(dirRemote)
	if err != nil {
		vfsError(w, dirRemote, err)
		return
	}
	dir, ok := node.(*vfs.Dir)
	if !ok {
		httpError(w, http.StatusNotFound, "Not a directory")
		return
	}

	var (
		name string
		body func(io.Writer) error
	)
	if query.Get("zip") == "multi" {
		rels, status, msg := parseSelection(query.Get("paths"), u.opt.MultiZipMax)
		if status != http.StatusOK {
			httpError(w, status, msg)
			return
		}
		for _, rel := range rels {
			full := path.Join(dirRemote, rel)
			// Each path was checked to have no dot segments, so it stays
			// below dirRemote, but the server's check also applies.
			if u.cfg.CheckPath("/"+full) != nil {
				httpError(w, http.StatusBadRequest, "Invalid selection path")
				return
			}
			if _, err := VFS.Stat(full); err != nil {
				httpError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
				return
			}
		}
		name = "selection.zip"
		body = func(out io.Writer) error { return writeSelectionZip(ctx, out, VFS, dirRemote, rels) }
	} else {
		base := path.Base(dirRemote)
		if dirRemote == "" {
			base = "root"
		}
		name = base + ".zip"
		// vfs.CreateZip isn't used as it names the entries below the second
		// level of directories with their full paths.
		children, err := dir.ReadDirAll()
		if err != nil {
			serve.Error(ctx, dirRemote, w, "Failed to list directory", err)
			return
		}
		rels := make([]string, len(children))
		for i, child := range children {
			rels[i] = child.Name()
		}
		body = func(out io.Writer) error { return writeSelectionZip(ctx, out, VFS, dirRemote, rels) }
	}

	header := w.Header()
	header.Set("Content-Type", "application/zip")
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	header.Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	fs.Infof(dirRemote, "%s: Zipping %s", r.RemoteAddr, name)
	if err := body(w); err != nil {
		fs.Errorf(dirRemote, "webui: zip failed: %v", err)
	}
}

// writeSelectionZip writes to w a zip of the entries rels of the directory
// dirRemote of the VFS
//
// The entries are named by their paths in rels, with directories followed
// by their contents. The paths must have been validated. If it fails the
// zip is left without its central directory so that it can't be mistaken
// for a complete one.
func writeSelectionZip(ctx context.Context, w io.Writer, VFS *vfs.VFS, dirRemote string, rels []string) error {
	zipWriter := zip.NewWriter(w)
	for _, rel := range rels {
		node, err := VFS.Stat(path.Join(dirRemote, rel))
		if err != nil {
			return fmt.Errorf("create zip stat: %w", err)
		}
		if err := addToZip(ctx, zipWriter, node, rel); err != nil {
			return err
		}
	}
	return zipWriter.Close()
}

// addToZip adds the node to the zip as name, and if it is a directory
// everything below it as name/child
func addToZip(ctx context.Context, zipWriter *zip.Writer, node vfs.Node, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch e := node.(type) {
	case *vfs.File:
		return addFileToZip(zipWriter, e, name)
	case *vfs.Dir:
		_, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     name + "/",
			Method:   zip.Store,
			Modified: e.ModTime(),
		})
		if err != nil {
			return fmt.Errorf("create zip directory header: %w", err)
		}
		children, err := e.ReadDirAll()
		if err != nil {
			return fmt.Errorf("create zip directory read: %w", err)
		}
		for _, child := range children {
			if err := addToZip(ctx, zipWriter, child, name+"/"+child.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

// addFileToZip adds the file to the zip as name
func addFileToZip(zipWriter *zip.Writer, file *vfs.File, name string) (err error) {
	in, err := file.Open(os.O_RDONLY)
	if err != nil {
		return fmt.Errorf("create zip open file: %w", err)
	}
	defer fs.CheckClose(in, &err)
	fileWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: file.ModTime(),
	})
	if err != nil {
		return fmt.Errorf("create zip file header: %w", err)
	}
	if _, err = io.Copy(fileWriter, in); err != nil {
		return fmt.Errorf("create zip copy: %w", err)
	}
	return nil
}
