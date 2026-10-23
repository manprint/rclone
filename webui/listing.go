package webui

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/vfs"
	"golang.org/x/sync/errgroup"
)

// maxDirCount caps the child count reported for a directory
const maxDirCount = 1000

// dirCountTimeout is how long a listing waits for the child counts of its
// directories before showing the missing ones as unknown
var dirCountTimeout = 5 * time.Second

// pageHeaders are the security headers of the HTML pages
var pageHeaders = map[string]string{
	"Cache-Control":           "no-cache",
	"X-Content-Type-Options":  "nosniff",
	"Content-Security-Policy": "frame-ancestors 'self'; object-src 'none'; base-uri 'self'",
}

// textBrowsers are the lowercased User-Agent prefixes of clients which
// send an HTML Accept header but want the plain directory listing
var textBrowsers = []string{
	"lynx/", "w3m/", "links ", "elinks/", "curl/", "wget/", "httpie/", "aria2/", "rclone/",
}

// wantsUI reports whether the request comes from a graphical browser
func wantsUI(r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return false
	}
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	for _, prefix := range textBrowsers {
		if strings.HasPrefix(ua, prefix) {
			return false
		}
	}
	return true
}

// serveListing handles GET and HEAD requests for the directory pages. It
// returns false if the request isn't one for the UI and must be handled
// by the next handler.
func (u *UI) serveListing(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	path := r.URL.Path
	query := r.URL.Query()
	isDir := path == "/" || strings.HasSuffix(path, "/")
	browser := wantsUI(r)

	// Decide from the request alone whether it is ours so that other
	// requests don't pay for a stat.
	switch {
	case !isDir:
		if r.Method != http.MethodGet || !browser {
			return false
		}
	case query.Has("download"):
		return false
	case query.Has("zip"):
		// Opened from links so it needs no browser
	case query.Has("disk-stats"):
		// Fetched by the page's script
	case !query.Has("json") && !browser:
		return false
	}

	if u.cfg.CheckPath(path) != nil {
		return false
	}
	ctx := r.Context()
	remote := strings.Trim(path, "/")
	VFS, err := u.cfg.GetVFS(ctx)
	if err != nil {
		return false
	}
	node, err := u.lookups.stat(ctx, VFS, remote)
	if errors.Is(err, vfs.ENOENT) {
		return false
	}
	if err != nil {
		// Passing on would make the next handler repeat the lookup
		vfsError(w, remote, err)
		return true
	}
	if !node.IsDir() {
		return false
	}

	if !isDir {
		// The page resolves its links against the current URL so it needs
		// the slash. Built from remote so that it can't be an absolute URL.
		loc := url.URL{
			Path:     strings.TrimSuffix(u.baseSlash(), "/") + "/" + remote + "/",
			RawQuery: r.URL.RawQuery,
		}
		http.Redirect(w, r, loc.String(), http.StatusFound)
		return true
	}

	if query.Has("zip") {
		u.serveZip(w, r, VFS, remote, query)
		return true
	}

	var (
		items            []PathItem
		searchIncomplete bool
	)
	needle := strings.ToLower(strings.TrimSpace(query.Get("q")))
	switch {
	case u.opt.Search && needle != "":
		items, searchIncomplete = u.searchItems(ctx, VFS, node.(*vfs.Dir), remote, needle)
		if items == nil {
			items = []PathItem{}
		}
	case query.Has("disk-stats"):
		u.serveDiskStats(w, r, VFS)
		return true
	default:
		entries, err := u.lookups.readDir(ctx, node.(*vfs.Dir))
		if err != nil {
			vfsError(w, remote, err)
			return true
		}
		items = make([]PathItem, 0, len(entries))
		for _, entry := range entries {
			items = append(items, u.pathItem(VFS, entry, remote))
		}
		u.fillDirCounts(ctx, entries, items)
	}
	sortPathItems(items, query.Get("sort"), query.Get("order"), query.Has("sort"))
	publinkEnabled := u.publinksEnabled() && u.allowed(remote)
	for i := 0; i < len(items) && !publinkEnabled; i++ {
		publinkEnabled = items[i].PublinkToken != nil
	}

	var user *string
	if name, ok := u.lookupUser(r); ok {
		user = &name
	}
	data := IndexData{
		Href:                          "/" + remote,
		Kind:                          "Index",
		URIPrefix:                     u.baseSlash(),
		AllowUpload:                   !VFS.Opt.ReadOnly,
		AllowDelete:                   !VFS.Opt.ReadOnly && u.opt.AllowDelete,
		AllowSearch:                   u.opt.Search,
		AllowArchive:                  !u.cfg.DisableZip,
		DirExists:                     true,
		Auth:                          u.cfg.UsingAuth(),
		User:                          user,
		Paths:                         items,
		PlaybackResumeEnabled:         u.resume != nil,
		PublinkEnabled:                publinkEnabled,
		EnableDataStats:               u.opt.DiskStats,
		ShowConcurrentUploadsSettings: u.opt.UploadSettings,
		SearchIncomplete:              searchIncomplete,
	}
	u.writeData(w, r, remote, &data, query.Has("json"))
	return true
}

// writeData replies to a GET or HEAD request with data, as JSON if asJSON
// is set and otherwise as the page with the data embedded in it
func (u *UI) writeData(w http.ResponseWriter, r *http.Request, remote string, data any, asJSON bool) {
	body, err := json.Marshal(data)
	if err != nil {
		serve.Error(r.Context(), remote, w, "Failed to encode page data", err)
		return
	}
	header := w.Header()
	if asJSON {
		header.Set("Content-Type", "application/json")
		header.Set("Cache-Control", "no-cache")
	} else {
		for k, v := range pageHeaders {
			header.Set(k, v)
		}
		header.Set("Content-Type", "text/html; charset=utf-8")
		page := strings.ReplaceAll(u.rawTpl, "__ASSETS_PREFIX__", u.baseSlash()+assetsPrefix)
		body = []byte(strings.Replace(page, "__INDEX_DATA__", base64.StdEncoding.EncodeToString(body), 1))
	}
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

// pathItem returns the listing entry for the node, which is in the
// directory dir of the VFS
func (u *UI) pathItem(VFS *vfs.VFS, entry vfs.Node, dir string) PathItem {
	item := u.entryItem(VFS, entry, dir)
	item.PublinkToken = u.publinkToken(path.Join(dir, item.Name))
	return item
}

// entryItem is pathItem without the public link, and with a size of -1 for
// a directory which fillDirCounts may replace
func (u *UI) entryItem(VFS *vfs.VFS, entry vfs.Node, dir string) PathItem {
	item := PathItem{
		PathType: PathTypeFile,
		Name:     entry.Name(),
		Size:     entry.Size(),
	}
	if !VFS.Opt.NoModTime {
		item.Mtime = entry.ModTime().UnixMilli()
	}
	if entry.IsDir() {
		item.PathType = PathTypeDir
		item.Size = -1
	} else {
		item.ResumePosition = u.resumePosition(VFS, path.Join(dir, item.Name))
	}
	return item
}

// fillDirCounts sets the size of the items of directories to the number of
// their children (up to maxDirCount) if the options ask for it. items[i]
// must be the entry of nodes[i].
//
// The directories are listed in parallel and those which can't be listed
// within dirCountTimeout keep the size -1, so that a remote which hangs
// doesn't hold up the listing.
func (u *UI) fillDirCounts(ctx context.Context, nodes []vfs.Node, items []PathItem) {
	if !u.opt.DirCount {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, dirCountTimeout)
	defer cancel()
	var g errgroup.Group
	g.SetLimit(max(1, fs.GetConfig(ctx).Checkers))
	for i, node := range nodes {
		dir, ok := node.(*vfs.Dir)
		if !ok {
			continue
		}
		g.Go(func() error {
			children, err := u.lookups.readDir(ctx, dir)
			if err != nil {
				// Not dir.Path(), which waits while the directory is
				// listed with its lock held
				fs.Debugf(items[i].Name, "webui: failed to count directory entries: %v", err)
				return nil
			}
			items[i].Size = int64(min(len(children), maxDirCount))
			return nil
		})
	}
	_ = g.Wait()
}
