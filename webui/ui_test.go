package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestUI returns a UI with a minimal config and the given base URL
func newTestUI(t *testing.T, baseURL string) *UI {
	t.Helper()
	u, err := New(Config{BaseURL: func() string { return baseURL }}, Options{})
	require.NoError(t, err)
	return u
}

// fallthroughHandler records the requests that reach the next handler
type fallthroughHandler struct {
	paths []string
}

func (h *fallthroughHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.paths = append(h.paths, r.URL.Path)
	w.WriteHeader(http.StatusTeapot)
}

func do(h http.Handler, method, target string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for i := 0; i < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestAssetsMatchSource checks the embedded assets are the ones recorded in SOURCE
func TestAssetsMatchSource(t *testing.T) {
	source, err := os.ReadFile("assets/SOURCE")
	require.NoError(t, err)
	assert.NotContains(t, string(source), "@", "SOURCE must not contain credentials")
	want := map[string]string{}
	for line := range strings.SplitSeq(string(source), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "sha256" {
			want[fields[1]] = fields[2]
		}
	}
	require.Len(t, want, 4)
	for _, name := range []string{"index.html", "index.js", "index.css", "favicon.ico"} {
		data, err := assetFS.ReadFile("assets/" + name)
		require.NoError(t, err, name)
		sum := sha256.Sum256(data)
		assert.Equal(t, want[name], hex.EncodeToString(sum[:]), name)
	}
}

// TestAssetsServed checks the asset handler and where it sits in the URL space
func TestAssetsServed(t *testing.T) {
	prefix := "/" + assetsPrefix
	t.Run("Root", func(t *testing.T) {
		next := &fallthroughHandler{}
		h := newTestUI(t, "/").PreAuth(next)

		rec := do(h, "GET", prefix+"index.js")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/javascript; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Equal(t, servedAssets["index.js"].data, rec.Body.Bytes())
		etag := rec.Header().Get("ETag")
		assert.NotEmpty(t, etag)
		assert.Contains(t, rec.Header().Get("Cache-Control"), "immutable")
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))

		rec = do(h, "GET", prefix+"index.js", "If-None-Match", etag)
		assert.Equal(t, http.StatusNotModified, rec.Code)
		assert.Empty(t, rec.Body.Bytes())

		rec = do(h, "HEAD", prefix+"index.css")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "text/css; charset=utf-8", rec.Header().Get("Content-Type"))
		assert.Empty(t, rec.Body.Bytes())

		assert.Equal(t, http.StatusOK, do(h, "GET", prefix+"favicon.ico").Code)
		assert.Equal(t, http.StatusOK, do(h, "GET", prefix+"overlay.js").Code)
		assert.Equal(t, http.StatusNotFound, do(h, "GET", prefix+"nope.js").Code)
		assert.Equal(t, http.StatusMethodNotAllowed, do(h, "POST", prefix+"index.js").Code)
		assert.Empty(t, next.paths)

		// Everything else is left to the next handler
		assert.Equal(t, http.StatusTeapot, do(h, "GET", "/index.js").Code)
		assert.Equal(t, []string{"/index.js"}, next.paths)
	})
	t.Run("BaseURL", func(t *testing.T) {
		next := &fallthroughHandler{}
		h := newTestUI(t, "/base").PreAuth(next)

		assert.Equal(t, http.StatusOK, do(h, "GET", "/base"+prefix+"index.css").Code)
		assert.Empty(t, next.paths)

		assert.Equal(t, http.StatusTeapot, do(h, "GET", prefix+"index.css").Code)
		assert.Equal(t, http.StatusTeapot, do(h, "GET", "/basement"+prefix+"index.css").Code)
		assert.Equal(t, []string{prefix + "index.css", "/basement" + prefix + "index.css"}, next.paths)
	})
	t.Run("UnnormalizedBaseURL", func(t *testing.T) {
		h := newTestUI(t, "base/").PreAuth(&fallthroughHandler{})
		assert.Equal(t, http.StatusOK, do(h, "GET", "/base"+prefix+"index.css").Code)
	})
	t.Run("ResponseHeaders", func(t *testing.T) {
		u, err := New(Config{
			BaseURL:         func() string { return "/" },
			AllowOrigin:     "http://example.com",
			ResponseHeaders: []string{"X-Test: yes"},
		}, Options{})
		require.NoError(t, err)
		rec := do(u.PreAuth(&fallthroughHandler{}), "GET", prefix+"index.js")
		assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "yes", rec.Header().Get("X-Test"))
	})
}

// TestOverlayInjected checks the overlay script is loaded right after index.js
func TestOverlayInjected(t *testing.T) {
	u := newTestUI(t, "/")
	index := strings.Index(u.rawTpl, "__ASSETS_PREFIX__index.js")
	overlay := strings.Index(u.rawTpl, "__ASSETS_PREFIX__overlay.js")
	require.GreaterOrEqual(t, index, 0)
	assert.Greater(t, overlay, index)
	assert.Equal(t, 1, strings.Count(u.rawTpl, "overlay.js"))
	assert.NotContains(t, indexHTML, "overlay.js", "the embedded template must stay as vendored")
}
