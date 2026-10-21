package webui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rclone/rclone/fs"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs"
)

// overlayTag is the script tag of the page template after which the
// overlay script is injected
const overlayTag = `<script src="__ASSETS_PREFIX__index.js"></script>`

// Config connects the web UI to the server it is attached to
type Config struct {
	BaseURL         func() string // "", "/" or "/prefix", read on every request
	GetVFS          func(ctx context.Context) (*vfs.VFS, error)
	HoldVFS         func(http.Handler) http.Handler
	CheckPath       func(name string) error
	UsingAuth       func() bool
	Realm           string
	DisableZip      bool
	AllowOrigin     string
	ResponseHeaders []string
	IsProxy         bool
}

// UI is a web UI attached to a server
type UI struct {
	cfg        Config
	opt        Options
	rawTpl     string                          // page template with the overlay script injected
	publicWrap func(http.Handler) http.Handler // CORS and response headers for requests handled before authentication
}

// New creates a web UI for the server described by cfg
func New(cfg Config, opt Options) (*UI, error) {
	if n := strings.Count(indexHTML, overlayTag); n != 1 {
		return nil, fmt.Errorf("incompatible assets: found %d script tags to inject the overlay after, want 1", n)
	}
	headers, err := fs.ParseHeaders(cfg.ResponseHeaders)
	if err != nil {
		return nil, err
	}
	if cfg.BaseURL == nil {
		return nil, errors.New("no BaseURL in config")
	}
	cors := libhttp.MiddlewareCORS(cfg.AllowOrigin)
	respHeaders := libhttp.MiddlewareResponseHeaders(headers)
	return &UI{
		cfg: cfg,
		opt: opt,
		rawTpl: strings.Replace(indexHTML, overlayTag,
			overlayTag+"\n  <script src=\"__ASSETS_PREFIX__overlay.js\"></script>", 1),
		publicWrap: func(h http.Handler) http.Handler {
			return cors(respHeaders(h))
		},
	}, nil
}

// baseSlash returns the base URL with a leading and trailing slash
//
// The server's BaseURL isn't normalized when the UI is created so this
// accepts any of "", "/", "prefix" or "/prefix/".
func (u *UI) baseSlash() string {
	b := strings.Trim(u.cfg.BaseURL(), "/")
	if b == "" {
		return "/"
	}
	return "/" + b + "/"
}

// PreAuth returns the handler for requests which need no authentication
//
// This is the assets. Everything else is passed to next.
func (u *UI) PreAuth(next http.Handler) http.Handler {
	assets := u.publicWrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, r, strings.TrimPrefix(u.relPath(r), "/"+assetsPrefix))
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(u.relPath(r), "/"+assetsPrefix) {
			assets.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler returns the handler for authenticated requests
//
// It serves the directory, edit and view pages and the operations of
// browsers, and sandboxes the files which can run scripts. Everything
// else is passed to next.
func (u *UI) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u.cfg.CheckPath(r.URL.Path) != nil {
			next.ServeHTTP(w, r)
			return
		}
		if u.serveWrite(w, r) || u.serveEdit(w, r) || u.serveListing(w, r) {
			return
		}
		sandboxActive(w, r)
		next.ServeHTTP(w, r)
	})
}

// relPath returns the request path with the base URL removed, or "" if
// the path isn't below the base URL
func (u *UI) relPath(r *http.Request) string {
	base := strings.TrimSuffix(u.baseSlash(), "/")
	rel, ok := strings.CutPrefix(r.URL.Path, base)
	if !ok {
		return ""
	}
	return rel
}
