package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// itemFunc returns the listing entry for a search hit, which is entry in
// the directory relDir, a path relative to the directory searched ("" for
// that directory itself)
type itemFunc func(entry vfs.Node, relDir string) PathItem

// search returns the entries below dir whose name contains needle, which
// must be lowercase, ignoring case.
//
// It reads directories breadth first. Those at more than maxDepth levels
// below dir aren't read and it stops at maxResults hits or when ctx is
// done, returning the hits found so far. A directory which can't be read
// is skipped. unread is the number of directories within maxDepth which
// weren't read because they failed or ctx was done first.
func search(ctx context.Context, l *lookups, dir *vfs.Dir, needle string, maxDepth, maxResults int, newItem itemFunc) (results []PathItem, unread int) {
	type queued struct {
		dir   *vfs.Dir
		rel   string
		depth int
	}
	queue := []queued{{dir: dir}}
	for len(queue) > 0 && len(results) < maxResults {
		if ctx.Err() != nil {
			unread += len(queue)
			break
		}
		cur := queue[0]
		queue = queue[1:]
		entries, err := l.readDir(ctx, cur.dir)
		if err != nil {
			// Not cur.dir.Path() which waits for a listing which hangs
			fs.Debugf(cur.rel, "webui: search failed to list directory: %v", err)
			unread++
			continue
		}
		for _, entry := range entries {
			if ctx.Err() != nil || len(results) >= maxResults {
				break
			}
			if strings.Contains(strings.ToLower(entry.Name()), needle) {
				results = append(results, newItem(entry, cur.rel))
			}
			if sub, ok := entry.(*vfs.Dir); ok && cur.depth+1 <= maxDepth {
				queue = append(queue, queued{dir: sub, rel: path.Join(cur.rel, entry.Name()), depth: cur.depth + 1})
			}
		}
	}
	return results, unread
}

// searchItems returns the unsorted results of a search below the directory
// remote of the VFS for the lowercase needle, within the limits of the
// options, and whether some directories couldn't be searched
func (u *UI) searchItems(ctx context.Context, VFS *vfs.VFS, dir *vfs.Dir, remote, needle string) (items []PathItem, incomplete bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(u.opt.SearchTimeout))
	defer cancel()
	items, unread := search(ctx, &u.lookups, dir, needle, u.opt.SearchMaxDepth, u.opt.SearchMaxResults,
		func(entry vfs.Node, relDir string) PathItem {
			item := u.entryItem(VFS, entry, path.Join(remote, relDir))
			item.PublinkToken = u.publinkToken(path.Join(remote, relDir, item.Name))
			item.Name = path.Join(relDir, item.Name)
			return item
		})
	if unread > 0 {
		fs.Infof(remote, "webui: search incomplete: %d directories not read", unread)
	}
	return items, unread > 0
}

// diskStats is the reply of ?disk-stats, where an unknown value is null
type diskStats struct {
	Usage     *int64 `json:"disk_usage"`
	Total     *int64 `json:"disk_total"`
	Available *int64 `json:"disk_available"`
}

// serveDiskStats replies to a GET or HEAD request with the space used,
// total and available of the VFS, as far as the backend can tell
//
// It replies 404 if the option is off or nothing is known.
func (u *UI) serveDiskStats(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS) {
	if !u.opt.DiskStats {
		http.NotFound(w, r)
		return
	}
	total, used, free := VFS.Statfs()
	known := func(n int64) *int64 {
		if n < 0 {
			return nil
		}
		return &n
	}
	stats := diskStats{Usage: known(used), Total: known(total), Available: known(free)}
	header := w.Header()
	header.Set("Content-Type", "application/json")
	header.Set("Cache-Control", "no-cache")
	var body []byte
	if stats.Usage == nil && stats.Total == nil && stats.Available == nil {
		body = []byte(`{"error":"Disk usage unknown"}`)
		w.WriteHeader(http.StatusNotFound)
	} else {
		body, _ = json.Marshal(stats)
		w.WriteHeader(http.StatusOK)
	}
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}
