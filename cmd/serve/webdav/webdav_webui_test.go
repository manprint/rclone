// fork:webui

//go:build !windows && !darwin

package webdav

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fs.GlobalOptionsInit doesn't run in tests so the environment never
// reaches the option, but the upstream suite is run with RCLONE_WEBUI=false.
func init() {
	if v, ok := os.LookupEnv("RCLONE_WEBUI"); ok {
		if enabled, err := strconv.ParseBool(v); err == nil {
			webui.Opt.Enabled = enabled
		}
	}
}

// startWebUI starts a server with the web UI enabled on a temporary
// directory and returns its URL, which ends in a slash.
//
// mutate and uiMutate adjust the server and web UI options and vfsOpt
// replaces the default VFS options if not nil.
func startWebUI(t *testing.T, mutate func(*Options), uiMutate func(*webui.Options), vfsOpt *vfscommon.Options) (url string) {
	t.Helper()
	return startWebUIDir(t, t.TempDir(), mutate, uiMutate, vfsOpt)
}

// startWebUIDir is startWebUI serving the local directory dir
func startWebUIDir(t *testing.T, dir string, mutate func(*Options), uiMutate func(*webui.Options), vfsOpt *vfscommon.Options) (url string) {
	t.Helper()
	savedOpt := webui.Opt
	t.Cleanup(func() { webui.Opt = savedOpt })
	webui.Opt.Enabled = true
	if uiMutate != nil {
		uiMutate(&webui.Opt)
	}

	f, err := fs.NewFs(context.Background(), dir)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}
	if mutate != nil {
		mutate(&opt)
	}
	if vfsOpt == nil {
		vfsOpt = &vfscommon.Opt
	}

	w, err := newWebDAV(context.Background(), f, &opt, vfsOpt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		assert.NoError(t, w.Serve())
	}()
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
	})
	return w.server.URLs()[0]
}

// TestWebUIFlagsRegistered checks the web UI flags are on the command
func TestWebUIFlagsRegistered(t *testing.T) {
	for _, name := range []string{"webui", "webui-search-max-depth", "webui-secret"} {
		assert.NotNil(t, Command.Flags().Lookup(name), name)
	}
	// The flag's DefValue follows RCLONE_WEBUI so check the declared default
	for _, option := range webui.OptionsInfo {
		if option.Name == "webui" {
			assert.Equal(t, true, option.Default)
		}
	}
	assert.Equal(t, "true", Command.Flags().Lookup("webui").NoOptDefVal)
}

// TestWebUIAssetsReachable checks the assets are served by the real server
func TestWebUIAssetsReachable(t *testing.T) {
	get := func(t *testing.T, url string) *http.Response {
		t.Helper()
		resp, err := http.Get(url)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		return resp
	}

	t.Run("Default", func(t *testing.T) {
		url := startWebUI(t, nil, nil, nil)
		resp := get(t, url+webui.AssetsPrefix()+"index.js")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/javascript; charset=utf-8", resp.Header.Get("Content-Type"))
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "formatDirSizeParts")

		assert.Equal(t, http.StatusOK, get(t, url+webui.AssetsPrefix()+"overlay.js").StatusCode)
		assert.Equal(t, http.StatusNotFound, get(t, url+webui.AssetsPrefix()+"nope.js").StatusCode)
	})

	t.Run("NoAuthNeeded", func(t *testing.T) {
		url := startWebUI(t, func(opt *Options) {
			opt.Auth.BasicUser = testUser
			opt.Auth.BasicPass = testPass
		}, nil, nil)
		assert.Equal(t, http.StatusOK, get(t, url+webui.AssetsPrefix()+"index.css").StatusCode)
		// Other paths still need it
		assert.Equal(t, http.StatusUnauthorized, get(t, url).StatusCode)
	})

	t.Run("BaseURL", func(t *testing.T) {
		url := startWebUI(t, func(opt *Options) {
			opt.HTTP.BaseURL = "/pfx"
		}, nil, nil)
		assert.Equal(t, http.StatusOK, get(t, url+webui.AssetsPrefix()+"favicon.ico").StatusCode)
	})
}

// listingDir makes a directory with dir1 holding two files, a10.txt,
// a2.txt and the three byte x.txt
func listingDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dir1"), 0777))
	for name, content := range map[string]string{
		"dir1/f1.txt": "1",
		"dir1/f2.txt": "22",
		"a10.txt":     "a10",
		"a2.txt":      "a2",
		"x.txt":       "xyz",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0666))
	}
	return dir
}

// noRedirect is an HTTP client which returns redirects instead of
// following them
var noRedirect = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

const browserUA = "Mozilla/5.0"

// doRequest makes a request with the given headers and returns the
// response and its body
func doRequest(t *testing.T, method, url string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := noRedirect.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

// browserHeaders are the headers of a graphical browser opening a page
var browserHeaders = map[string]string{"Accept": "text/html,application/xhtml+xml", "User-Agent": browserUA}

var indexDataRe = regexp.MustCompile(`<template id="index-data">([^<]*)</template>`)

// pageData decodes the data embedded in an HTML page
func pageData(t *testing.T, body string) (data webui.IndexData) {
	t.Helper()
	m := indexDataRe.FindStringSubmatch(body)
	require.NotNil(t, m, "no index-data template in page")
	raw, err := base64.StdEncoding.DecodeString(m[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &data))
	return data
}

// jsonData decodes a ?json response
func jsonData(t *testing.T, body string) (data webui.IndexData) {
	t.Helper()
	require.NoError(t, json.Unmarshal([]byte(body), &data))
	return data
}

func names(items []webui.PathItem) (out []string) {
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}

// TestWebUIListingHTML checks a browser gets the page with the listing
func TestWebUIListingHTML(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), nil, nil, nil)
	resp, body := doRequest(t, "GET", url, browserHeaders)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "frame-ancestors 'self'; object-src 'none'; base-uri 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Contains(t, body, "overlay.js")
	assert.Contains(t, body, webui.AssetsPrefix()+"index.js")
	assert.NotContains(t, body, "__ASSETS_PREFIX__")
	assert.NotContains(t, body, "__INDEX_DATA__")
	assert.Contains(t, body, `<template id="index-data">`)

	data := pageData(t, body)
	assert.Equal(t, "Index", data.Kind)
	assert.Equal(t, "/", data.Href)
	assert.Equal(t, "/", data.URIPrefix)
	assert.True(t, data.AllowUpload)
	assert.True(t, data.AllowDelete)
	assert.True(t, data.AllowArchive)
	assert.True(t, data.DirExists)
	assert.False(t, data.Auth)
	assert.Nil(t, data.User)
	require.Len(t, data.Paths, 4)
	assert.Equal(t, []string{"dir1", "a2.txt", "a10.txt", "x.txt"}, names(data.Paths))
	assert.Equal(t, webui.PathTypeDir, data.Paths[0].PathType)
	assert.Equal(t, int64(-1), data.Paths[0].Size)
	assert.Equal(t, webui.PathTypeFile, data.Paths[3].PathType)
	assert.Equal(t, int64(3), data.Paths[3].Size)

	t.Run("Subdirectory", func(t *testing.T) {
		_, body := doRequest(t, "GET", url+"dir1/", browserHeaders)
		data := pageData(t, body)
		assert.Equal(t, "/dir1", data.Href)
		assert.Equal(t, []string{"f1.txt", "f2.txt"}, names(data.Paths))
	})

	t.Run("Head", func(t *testing.T) {
		resp, body := doRequest(t, "HEAD", url, browserHeaders)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Empty(t, body)
	})

	t.Run("BaseURL", func(t *testing.T) {
		url := startWebUIDir(t, listingDir(t), func(opt *Options) { opt.HTTP.BaseURL = "/pfx" }, nil, nil)
		_, body := doRequest(t, "GET", url, browserHeaders)
		assert.Contains(t, body, "/pfx/"+webui.AssetsPrefix()+"overlay.js")
		data := pageData(t, body)
		assert.Equal(t, "/", data.Href)
		assert.Equal(t, "/pfx/", data.URIPrefix)
	})

	t.Run("NotFound", func(t *testing.T) {
		resp, body := doRequest(t, "GET", url+"nope/", browserHeaders)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.NotContains(t, body, "index-data")
	})
}

// TestWebUIListingJSON checks ?json and the sort parameters
func TestWebUIListingJSON(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), nil, nil, nil)
	resp, body := doRequest(t, "GET", url+"?json", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	data := jsonData(t, body)
	assert.Equal(t, []string{"dir1", "a2.txt", "a10.txt", "x.txt"}, names(data.Paths))
	for _, item := range data.Paths {
		assert.Greater(t, item.Mtime, int64(1e12), item.Name)
	}

	_, body = doRequest(t, "GET", url+"?json&sort=size&order=desc", nil)
	assert.Equal(t, []string{"x.txt", "a10.txt", "a2.txt", "dir1"}, names(jsonData(t, body).Paths))

	_, body = doRequest(t, "GET", url+"?json&order=desc", nil)
	assert.Equal(t, []string{"dir1", "a2.txt", "a10.txt", "x.txt"}, names(jsonData(t, body).Paths), "order without sort is ignored")

	t.Run("EmptyDirectory", func(t *testing.T) {
		url := startWebUI(t, nil, nil, nil)
		_, body := doRequest(t, "GET", url+"?json", nil)
		assert.Contains(t, body, `"paths":[]`)
	})

	t.Run("DisableZip", func(t *testing.T) {
		url := startWebUIDir(t, listingDir(t), func(opt *Options) { opt.DisableZip = true }, nil, nil)
		_, body := doRequest(t, "GET", url+"?json", nil)
		assert.False(t, jsonData(t, body).AllowArchive)
	})

	t.Run("DownloadIsNotHandled", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", url+"dir1/?download=zip&json", nil)
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
	})

	t.Run("NoModTime", func(t *testing.T) {
		vfsOpt := vfscommon.Opt
		vfsOpt.NoModTime = true
		url := startWebUIDir(t, listingDir(t), nil, nil, &vfsOpt)
		_, body := doRequest(t, "GET", url+"?json", nil)
		for _, item := range jsonData(t, body).Paths {
			assert.Equal(t, int64(0), item.Mtime, item.Name)
		}
	})
}

// TestWebUICLIFallback checks command line clients get the upstream listing
func TestWebUICLIFallback(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), nil, nil, nil)
	for _, tc := range []struct {
		name   string
		header map[string]string
	}{
		{"Curl", map[string]string{"User-Agent": "curl/8.0", "Accept": "*/*"}},
		{"CurlWithHTMLAccept", map[string]string{"User-Agent": "curl/8.0", "Accept": "text/html"}},
		{"Rclone", map[string]string{"User-Agent": "rclone/v1.0", "Accept": "text/html"}},
		{"NoAccept", map[string]string{"User-Agent": browserUA}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := doRequest(t, "GET", url, tc.header)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "<title>/</title>")
			assert.NotContains(t, body, "index-data")
		})
	}
}

// TestWebUIDirRedirect checks directories opened without the slash
func TestWebUIDirRedirect(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		url := startWebUIDir(t, listingDir(t), nil, nil, nil)
		resp, _ := doRequest(t, "GET", url+"dir1", browserHeaders)
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/dir1/", resp.Header.Get("Location"))

		resp, _ = doRequest(t, "GET", url+"dir1?sort=size&order=desc", browserHeaders)
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/dir1/?sort=size&order=desc", resp.Header.Get("Location"))

		resp, _ = doRequest(t, "GET", url+"dir1", map[string]string{"User-Agent": "curl/8.0", "Accept": "*/*"})
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

		resp, _ = doRequest(t, "HEAD", url+"dir1", browserHeaders)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)

		// Not a directory
		resp, body := doRequest(t, "GET", url+"x.txt", browserHeaders)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "xyz", body)
	})

	t.Run("BaseURL", func(t *testing.T) {
		url := startWebUIDir(t, listingDir(t), func(opt *Options) { opt.HTTP.BaseURL = "/pfx" }, nil, nil)
		resp, _ := doRequest(t, "GET", url+"dir1", browserHeaders)
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/pfx/dir1/", resp.Header.Get("Location"))
	})

	t.Run("Escaping", func(t *testing.T) {
		dir := listingDir(t)
		require.NoError(t, os.Mkdir(filepath.Join(dir, "a b#c"), 0777))
		url := startWebUIDir(t, dir, nil, nil, nil)
		resp, _ := doRequest(t, "GET", url+"a%20b%23c", browserHeaders)
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/a%20b%23c/", resp.Header.Get("Location"))
	})

	t.Run("NoOpenRedirect", func(t *testing.T) {
		dir := listingDir(t)
		require.NoError(t, os.Mkdir(filepath.Join(dir, "evil.example"), 0777))
		url := startWebUIDir(t, dir, nil, nil, nil)
		resp, _ := doRequest(t, "GET", url+"/evil.example", browserHeaders)
		assert.Equal(t, http.StatusFound, resp.StatusCode)
		assert.Equal(t, "/evil.example/", resp.Header.Get("Location"))
	})
}

// TestWebUIReadOnly checks a read only VFS disables upload and delete
func TestWebUIReadOnly(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.ReadOnly = true
	url := startWebUIDir(t, listingDir(t), nil, nil, &vfsOpt)
	_, body := doRequest(t, "GET", url+"?json", nil)
	data := jsonData(t, body)
	assert.False(t, data.AllowUpload)
	assert.False(t, data.AllowDelete)
}

// TestWebUIDirCount checks --webui-dir-count
func TestWebUIDirCount(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), nil, func(o *webui.Options) { o.DirCount = true }, nil)
	_, body := doRequest(t, "GET", url+"?json", nil)
	data := jsonData(t, body)
	require.NotEmpty(t, data.Paths)
	assert.Equal(t, "dir1", data.Paths[0].Name)
	assert.Equal(t, int64(2), data.Paths[0].Size)
	assert.Equal(t, int64(3), data.Paths[3].Size, "files keep their size")
}

// TestWebUIDisabledByTemplate checks --template leaves the upstream listing
func TestWebUIDisabledByTemplate(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), func(opt *Options) { opt.Template.Path = testTemplate }, nil, nil)
	resp, body := doRequest(t, "GET", url, browserHeaders)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "Directory listing of /")
	assert.NotContains(t, body, "index-data")
	_, body = doRequest(t, "GET", url+"?json", nil)
	assert.NotContains(t, body, `"kind"`)
	resp, _ = doRequest(t, "GET", url+webui.AssetsPrefix()+"index.js", nil)
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestWebUIDisabledFlag checks --webui=false leaves the upstream listing
func TestWebUIDisabledFlag(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), nil, func(o *webui.Options) { o.Enabled = false }, nil)
	resp, body := doRequest(t, "GET", url, browserHeaders)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "<title>/</title>")
	assert.NotContains(t, body, "index-data")
}

// TestWebUIAuthUser checks the authenticated user is in the page data
func TestWebUIAuthUser(t *testing.T) {
	url := startWebUIDir(t, listingDir(t), func(opt *Options) {
		opt.Auth.BasicUser = testUser
		opt.Auth.BasicPass = testPass
	}, nil, nil)
	resp, _ := doRequest(t, "GET", url+"?json", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	req, err := http.NewRequest("GET", url+"?json", nil)
	require.NoError(t, err)
	req.SetBasicAuth(testUser, testPass)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := jsonData(t, string(body))
	assert.True(t, data.Auth)
	require.NotNil(t, data.User)
	assert.Equal(t, testUser, *data.User)
}

// TestWebUIUpstreamSuiteEnabled runs the upstream webdav suite with the
// web UI on.
//
// TestWebDav sets a template, which turns the UI off, so this copies its
// server setup without one.
func TestWebUIUpstreamSuiteEnabled(t *testing.T) {
	savedOpt := webui.Opt
	t.Cleanup(func() { webui.Opt = savedOpt })
	webui.Opt.Enabled = true

	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.HTTP.ListenAddr = []string{testBindAddress}
		opt.HTTP.BaseURL = "/prefix"
		opt.Auth.BasicUser = testUser
		opt.Auth.BasicPass = testPass
		opt.EtagHash = "MD5"

		w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
		require.NoError(t, err)
		require.NotNil(t, w.webui, "web UI not attached")
		go func() {
			require.NoError(t, w.Serve())
		}()

		config := configmap.Simple{
			"type":   "webdav",
			"vendor": "rclone",
			"url":    w.server.URLs()[0],
			"user":   testUser,
			"pass":   obscure.MustObscure(testPass),
		}

		return config, func() {
			assert.NoError(t, w.Shutdown())
		}
	}

	servetest.Run(t, "webdav", start)
}

// browserOrigin is the Origin header browsers send with requests made by scripts
var browserOrigin = map[string]string{"Origin": "http://localhost"}

// send makes a request with a body and returns the response and its body
func send(t *testing.T, method, url, body string, header map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := noRedirect.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(got)
}

// withOrigin returns a copy of header with the browser's Origin added
func withOrigin(header map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range browserOrigin {
		out[k] = v
	}
	for k, v := range header {
		out[k] = v
	}
	return out
}

// cacheWritesOpt returns VFS options with the cache on for writes, using a
// temporary cache directory
func cacheWritesOpt(t *testing.T) *vfscommon.Options {
	t.Helper()
	require.NoError(t, config.SetCacheDir(t.TempDir()))
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeWrites
	return &opt
}

// TestWebUICheckAuthLogout checks the CHECKAUTH and LOGOUT requests
func TestWebUICheckAuthLogout(t *testing.T) {
	t.Run("NoAuth", func(t *testing.T) {
		url := startWebUI(t, nil, nil, nil)
		resp, body := doRequest(t, "CHECKAUTH", url, nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Equal(t, "", body)

		resp, _ = doRequest(t, "CHECKAUTH", url+"?login", nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `realm="rclone"`)

		resp, body = doRequest(t, "LOGOUT", url, nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), `realm="rclone"`)
		assert.Equal(t, "", body)
	})
	t.Run("Auth", func(t *testing.T) {
		url := startWebUI(t, func(opt *Options) {
			opt.Auth.BasicUser = testUser
			opt.Auth.BasicPass = testPass
			opt.Auth.Realm = "myrealm"
		}, nil, nil)

		// The upstream authentication answers requests without credentials
		resp, _ := doRequest(t, "CHECKAUTH", url, nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		req, err := http.NewRequest("CHECKAUTH", url, nil)
		require.NoError(t, err)
		req.SetBasicAuth(testUser, testPass)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, testUser, string(body))

		req, err = http.NewRequest("LOGOUT", url, nil)
		require.NoError(t, err)
		req.SetBasicAuth(testUser, testPass)
		resp2, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp2.Body.Close() }()
		assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
		assert.Contains(t, resp2.Header.Get("WWW-Authenticate"), `realm="myrealm"`)
	})
}

// TestWebUIPutCreatesParents checks a PUT from a browser creates the
// parent directories where the upstream server answers 409
func TestWebUIPutCreatesParents(t *testing.T) {
	dir := t.TempDir()
	url := startWebUIDir(t, dir, nil, nil, nil)

	resp, _ := send(t, "PUT", url+"new/sub/f.txt", "hello", withOrigin(nil))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	got, err := os.ReadFile(filepath.Join(dir, "new", "sub", "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))

	// Overwriting replaces the contents
	resp, _ = send(t, "PUT", url+"new/sub/f.txt", "hi", withOrigin(nil))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	got, err = os.ReadFile(filepath.Join(dir, "new", "sub", "f.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hi", string(got))

	// An empty file, as made by the UI for a new file
	resp, _ = send(t, "PUT", url+"empty.txt", "", withOrigin(nil))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	st, err := os.Stat(filepath.Join(dir, "empty.txt"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), st.Size())

	// A directory is left to the upstream handler
	resp, _ = send(t, "PUT", url+"new", "x", withOrigin(nil))
	assert.GreaterOrEqual(t, resp.StatusCode, 400)
	assert.DirExists(t, filepath.Join(dir, "new"))

	// Without Origin it is the upstream behaviour
	resp, _ = send(t, "PUT", url+"other/sub/f.txt", "hello", nil)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.NoDirExists(t, filepath.Join(dir, "other"))
}

// TestWebUIPutReadOnly checks a PUT from a browser can't write to a read only VFS
func TestWebUIPutReadOnly(t *testing.T) {
	dir := t.TempDir()
	opt := vfscommon.Opt
	opt.ReadOnly = true
	url := startWebUIDir(t, dir, nil, nil, &opt)
	resp, _ := send(t, "PUT", url+"new/f.txt", "hello", withOrigin(nil))
	assert.GreaterOrEqual(t, resp.StatusCode, 400)
	assert.NoDirExists(t, filepath.Join(dir, "new"))
}

// TestPutIncompleteRemovedNoCache checks a PUT from a browser which ends
// early leaves no file behind when the VFS has no cache
func TestPutIncompleteRemovedNoCache(t *testing.T) {
	dir := t.TempDir()
	url := startWebUIDir(t, dir, nil, nil, nil)

	conn, err := net.Dial("tcp", strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/"))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	_, err = io.WriteString(conn, "PUT /inc.bin HTTP/1.1\r\nHost: localhost\r\nOrigin: http://x\r\n"+
		"Content-Length: 10\r\n\r\nabc")
	require.NoError(t, err)
	// The server sees the end of the body but can still answer
	require.NoError(t, conn.(*net.TCPConn).CloseWrite())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	reply, _ := io.ReadAll(conn)
	assert.Contains(t, string(reply), "400 Bad Request")

	// The client sees the error before the handler finishes removing the file
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "inc.bin"))
		return os.IsNotExist(err)
	}, 2*time.Second, 20*time.Millisecond)
}

// TestPatchAppend checks a PATCH from a browser appends to a file
func TestPatchAppend(t *testing.T) {
	get := func(t *testing.T, url string) string {
		t.Helper()
		resp, body := doRequest(t, "GET", url, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return body
	}
	appendHeader := withOrigin(map[string]string{"X-Update-Range": "append"})

	t.Run("Writes", func(t *testing.T) {
		url := startWebUI(t, nil, nil, cacheWritesOpt(t))
		resp, _ := send(t, "PUT", url+"a.bin", "abc", withOrigin(nil))
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		resp, _ = send(t, "PATCH", url+"a.bin", "def", appendHeader)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "abcdef", get(t, url+"a.bin"))

		resp, _ = send(t, "PATCH", url+"a.bin", "ghi", withOrigin(map[string]string{"X-Update-Range": "bytes=0-1"}))
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "abcdef", get(t, url+"a.bin"))

		resp, _ = send(t, "PATCH", url+"missing.bin", "x", appendHeader)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		resp, _ = send(t, "MKCOL", url+"d", "", nil)
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		resp, _ = send(t, "PATCH", url+"d", "x", appendHeader)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
	t.Run("IncompleteKept", func(t *testing.T) {
		url := startWebUI(t, nil, nil, cacheWritesOpt(t))
		conn, err := net.Dial("tcp", strings.TrimSuffix(strings.TrimPrefix(url, "http://"), "/"))
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		_, err = io.WriteString(conn, "PUT /part.bin HTTP/1.1\r\nHost: localhost\r\nOrigin: http://x\r\n"+
			"Content-Length: 10\r\n\r\nabc")
		require.NoError(t, err)
		require.NoError(t, conn.(*net.TCPConn).CloseWrite())
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		reply, _ := io.ReadAll(conn)
		assert.Contains(t, string(reply), "400 Bad Request")

		// The bytes received stay so that the upload can be resumed
		require.Eventually(t, func() bool {
			resp, body := doRequest(t, "GET", url+"part.bin", nil)
			return resp.StatusCode == http.StatusOK && body == "abc"
		}, 2*time.Second, 20*time.Millisecond)
		resp, _ := send(t, "PATCH", url+"part.bin", "defghij", appendHeader)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "abcdefghij", get(t, url+"part.bin"))
	})
	t.Run("NoCache", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), []byte("abc"), 0666))
		url := startWebUIDir(t, dir, nil, nil, nil)
		resp, _ := send(t, "PATCH", url+"a.bin", "def", appendHeader)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
		assert.Equal(t, "abc", get(t, url+"a.bin"))
	})
	t.Run("NoOrigin", func(t *testing.T) {
		url := startWebUI(t, nil, nil, cacheWritesOpt(t))
		resp, _ := send(t, "PUT", url+"a.bin", "abc", nil)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		resp, _ = send(t, "PATCH", url+"a.bin", "def", map[string]string{"X-Update-Range": "append"})
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
		assert.Equal(t, "abc", get(t, url+"a.bin"))
	})
}

// moveFixture makes src.txt and the directory dst holding keep.txt
func moveFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dst"), 0777))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dst", "keep.txt"), []byte("keep"), 0666))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src.txt"), []byte("src"), 0666))
	return dir
}

// TestMoveGuardBrowser checks a MOVE from a browser doesn't replace a directory
func TestMoveGuardBrowser(t *testing.T) {
	for _, base := range []string{"", "/pfx"} {
		t.Run("BaseURL"+base, func(t *testing.T) {
			dir := moveFixture(t)
			url := startWebUIDir(t, dir, func(opt *Options) { opt.HTTP.BaseURL = base }, nil, nil)

			// Onto a directory, with or without the slash
			for _, dst := range []string{"dst", "dst/"} {
				resp, body := send(t, "MOVE", url+"src.txt", "", withOrigin(map[string]string{"Destination": url + dst}))
				assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
				assert.Equal(t, "Destination folder exists\n", body)
				assert.FileExists(t, filepath.Join(dir, "dst", "keep.txt"))
				assert.FileExists(t, filepath.Join(dir, "src.txt"))
			}

			// A path without the base URL doesn't name the directory
			if base != "" {
				resp, _ := send(t, "MOVE", url+"src.txt", "", withOrigin(map[string]string{"Destination": "/dst"}))
				assert.NotEqual(t, http.StatusPreconditionFailed, resp.StatusCode)
				assert.FileExists(t, filepath.Join(dir, "dst", "keep.txt"))
			}

			// An explicit Overwrite is the client's decision
			resp, _ := send(t, "MOVE", url+"src.txt", "", withOrigin(map[string]string{
				"Destination": url + "dst", "Overwrite": "F"}))
			assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode)
			assert.FileExists(t, filepath.Join(dir, "dst", "keep.txt"))

			// To a new name is handled upstream
			resp, _ = send(t, "MOVE", url+"src.txt", "", withOrigin(map[string]string{"Destination": url + "new.txt"}))
			assert.Contains(t, []int{http.StatusCreated, http.StatusNoContent}, resp.StatusCode)
			assert.FileExists(t, filepath.Join(dir, "new.txt"))
			assert.NoFileExists(t, filepath.Join(dir, "src.txt"))
		})
	}
}

// TestWebUIDavClientUnchanged checks requests without an Origin header get
// the same status from the server with the UI as without it
func TestWebUIDavClientUnchanged(t *testing.T) {
	run := func(t *testing.T, uiOn bool) (statuses []int) {
		dir := moveFixture(t)
		url := startWebUIDir(t, dir, nil, func(opt *webui.Options) { opt.Enabled = uiOn }, cacheWritesOpt(t))
		for _, req := range []struct {
			method, path, body string
			header             map[string]string
		}{
			{"PUT", "missing/sub/f.txt", "hello", nil},
			{"PUT", "a.bin", "abc", nil},
			{"PATCH", "a.bin", "def", map[string]string{"X-Update-Range": "append"}},
			{"PATCH", "a.bin", "def", nil},
			{"MOVE", "src.txt", "", map[string]string{"Destination": url + "dst"}},
			{"MOVE", "a.bin", "", map[string]string{"Destination": url + "b.bin"}},
			{"GET", "b.bin", "", nil},
			{"GET", "dst/keep.txt", "", nil},
		} {
			resp, _ := send(t, req.method, url+req.path, req.body, req.header)
			statuses = append(statuses, resp.StatusCode)
		}
		return statuses
	}
	with := run(t, true)
	without := run(t, false)
	assert.Equal(t, without, with)
	// The first request is the one the UI changes for browsers
	assert.Equal(t, http.StatusConflict, with[0])
}

// TestWebUIPathTraversal checks the requests from browsers are held to the
// server's path check
func TestWebUIPathTraversal(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	require.NoError(t, os.Mkdir(root, 0777))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src.txt"), []byte("src"), 0666))
	url := startWebUIDir(t, root, nil, nil, nil)

	resp, _ := send(t, "PUT", url+"a/%2e%2e/%2e%2e/x.txt", "x", withOrigin(nil))
	assert.False(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "status %d", resp.StatusCode)

	resp, _ = send(t, "PATCH", url+"a/%2e%2e/%2e%2e/x.txt", "x", withOrigin(map[string]string{"X-Update-Range": "append"}))
	assert.False(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "status %d", resp.StatusCode)

	resp, _ = send(t, "MOVE", url+"src.txt", "", withOrigin(map[string]string{"Destination": url + "%2e%2e/x"}))
	assert.False(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "status %d", resp.StatusCode)

	assert.NoFileExists(t, filepath.Join(outer, "x.txt"))
	assert.NoFileExists(t, filepath.Join(outer, "x"))
	assert.NoFileExists(t, filepath.Join(root, "x.txt"))
	assert.FileExists(t, filepath.Join(root, "src.txt"))
	entries, err := os.ReadDir(outer)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
