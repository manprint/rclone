package webui

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

//go:embed assets/index.html assets/index.js assets/index.css assets/favicon.ico overlay/overlay.js
var assetFS embed.FS

// asset is an embedded file served to browsers
type asset struct {
	contentType string
	data        []byte
	etag        string
}

// assetOrder is the order in which the embedded files are hashed into
// assetsPrefix
var assetOrder = []string{
	"assets/index.html", "assets/index.js", "assets/index.css",
	"assets/favicon.ico", "overlay/overlay.js",
}

var (
	// indexHTML is the raw page template shipped with the web UI
	indexHTML string

	// assetsPrefix is the path segment under which the assets are served.
	//
	// It contains a hash of all the assets so a new version of any of
	// them is fetched by browsers despite the immutable caching.
	assetsPrefix string

	// servedAssets maps the served file name to its content
	servedAssets map[string]*asset
)

func init() {
	h := sha256.New()
	files := make(map[string][]byte, len(assetOrder))
	for _, name := range assetOrder {
		// The files are embedded at build time so reading can't fail
		data, err := assetFS.ReadFile(name)
		if err != nil {
			panic("webui: embedded asset missing: " + err.Error())
		}
		files[name] = data
		h.Write(data)
	}
	indexHTML = string(files["assets/index.html"])
	assetsPrefix = "__webui_" + hex.EncodeToString(h.Sum(nil))[:8] + "__/"

	newAsset := func(contentType, name string) *asset {
		sum := sha256.Sum256(files[name])
		return &asset{
			contentType: contentType,
			data:        files[name],
			etag:        `"` + hex.EncodeToString(sum[:])[:16] + `"`,
		}
	}
	servedAssets = map[string]*asset{
		"index.js":    newAsset("application/javascript; charset=utf-8", "assets/index.js"),
		"index.css":   newAsset("text/css; charset=utf-8", "assets/index.css"),
		"favicon.ico": newAsset("image/x-icon", "assets/favicon.ico"),
		"overlay.js":  newAsset("application/javascript; charset=utf-8", "overlay/overlay.js"),
	}
}

// AssetsPrefix returns the path segment, with a trailing slash, below
// which the assets are served
func AssetsPrefix() string {
	return assetsPrefix
}

// serveAsset serves the named embedded asset
func serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	a, ok := servedAssets[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	header := w.Header()
	header.Set("ETag", a.etag)
	header.Set("Cache-Control", "public, max-age=31536000, immutable")
	header.Set("X-Content-Type-Options", "nosniff")
	if etagMatches(r.Header.Get("If-None-Match"), a.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	header.Set("Content-Type", a.contentType)
	header.Set("Content-Length", strconv.Itoa(len(a.data)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(a.data)
	}
}

// etagMatches reports whether the If-None-Match header value lists etag
func etagMatches(ifNoneMatch, etag string) bool {
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
