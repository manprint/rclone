package webui

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/vfs"
)

// maxDirCount caps the child count reported for a directory
const maxDirCount = 1000

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
	node, err := VFS.Stat(remote)
	if err != nil || !node.IsDir() {
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

	var items []PathItem
	needle := strings.ToLower(strings.TrimSpace(query.Get("q")))
	switch {
	case u.opt.Search && needle != "":
		items = u.searchItems(ctx, VFS, node.(*vfs.Dir), remote, needle)
		if items == nil {
			items = []PathItem{}
		}
	case query.Has("disk-stats"):
		u.serveDiskStats(w, r, VFS)
		return true
	default:
		entries, err := node.(*vfs.Dir).ReadDirAll()
		if err != nil {
			serve.Error(ctx, remote, w, "Failed to list directory", err)
			return true
		}
		items = make([]PathItem, 0, len(entries))
		for _, entry := range entries {
			items = append(items, u.pathItem(VFS, entry, remote))
		}
	}
	sortPathItems(items, query.Get("sort"), query.Get("order"), query.Has("sort"))

	var user *string
	if name, ok := u.lookupUser(r); ok {
		user = &name
	}
	data := IndexData{
		Href:                          "/" + remote,
		Kind:                          "Index",
		URIPrefix:                     u.baseSlash(),
		AllowUpload:                   !VFS.Opt.ReadOnly,
		AllowDelete:                   !VFS.Opt.ReadOnly,
		AllowSearch:                   u.opt.Search,
		AllowArchive:                  !u.cfg.DisableZip,
		DirExists:                     true,
		Auth:                          u.cfg.UsingAuth(),
		User:                          user,
		Paths:                         items,
		PlaybackResumeEnabled:         u.resume != nil,
		EnableDataStats:               u.opt.DiskStats,
		ShowConcurrentUploadsSettings: u.opt.UploadSettings,
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
	return u.entryItem(VFS, entry, dir, true)
}

// entryItem is pathItem which counts the entries of a directory only if
// countDirs is set and the options ask for it
func (u *UI) entryItem(VFS *vfs.VFS, entry vfs.Node, dir string, countDirs bool) PathItem {
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
		if countDirs {
			item.Size = u.dirSize(entry)
		}
	} else {
		item.ResumePosition = u.resumePosition(VFS, path.Join(dir, item.Name))
	}
	return item
}

// dirSize returns the size shown for a directory: -1, or the number of
// its children (up to maxDirCount) if the option asks for it and the
// directory can be listed.
func (u *UI) dirSize(entry vfs.Node) int64 {
	if !u.opt.DirCount {
		return -1
	}
	dir, ok := entry.(*vfs.Dir)
	if !ok {
		return -1
	}
	children, err := dir.ReadDirAll()
	if err != nil {
		fs.Debugf(dir.Path(), "webui: failed to count directory entries: %v", err)
		return -1
	}
	return int64(min(len(children), maxDirCount))
}
