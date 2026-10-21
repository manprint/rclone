// fork:webui

//go:build !windows && !darwin

package webdav

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
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

// editData decodes the data embedded in an edit or view page
func editData(t *testing.T, body string) (data webui.EditData) {
	t.Helper()
	m := indexDataRe.FindStringSubmatch(body)
	require.NotNil(t, m, "no index-data template in page")
	raw, err := base64.StdEncoding.DecodeString(m[1])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &data))
	return data
}

// TestWebUIEditPage checks the edit and view pages of a file
func TestWebUIEditPage(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"note.txt": "ciao",
		"bin.dat":  "\x00\x01",
		"v.mp4":    "not really a video",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0666))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d"), 0777))
	url := startWebUIDir(t, dir, nil, nil, nil)

	t.Run("Edit", func(t *testing.T) {
		// Served to any client as the URL is an explicit one for the UI
		resp, body := doRequest(t, "GET", url+"note.txt?edit", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
		assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "frame-ancestors")
		data := editData(t, body)
		assert.Equal(t, "Edit", data.Kind)
		assert.Equal(t, "/note.txt", data.Href)
		assert.Equal(t, "/", data.URIPrefix)
		assert.True(t, data.Editable)
		assert.True(t, data.AllowUpload)
		assert.True(t, data.AllowDelete)
		assert.False(t, data.Auth)
		assert.Nil(t, data.User)
		assert.Equal(t, 30, data.ResilientPlayerMaxRetry)
		assert.Equal(t, 10, data.ResilientPlayerReconnectInterval)
	})

	t.Run("Head", func(t *testing.T) {
		resp, body := doRequest(t, "HEAD", url+"note.txt?edit", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.NotEmpty(t, resp.Header.Get("Content-Length"))
		assert.Empty(t, body)
	})

	t.Run("Binary", func(t *testing.T) {
		resp, body := doRequest(t, "GET", url+"bin.dat?edit", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.False(t, editData(t, body).Editable)
	})

	t.Run("View", func(t *testing.T) {
		resp, body := doRequest(t, "GET", url+"v.mp4?view", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		data := editData(t, body)
		assert.Equal(t, "View", data.Kind)
		assert.Equal(t, "/v.mp4", data.Href)
		// The server ignores playlist
		_, body = doRequest(t, "GET", url+"v.mp4?view&playlist", nil)
		assert.Equal(t, "View", editData(t, body).Kind)
	})

	t.Run("NotFound", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", url+"missing.txt?edit", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Directory", func(t *testing.T) {
		// Left to the listing, which ignores the query
		resp, body := doRequest(t, "GET", url+"d/?edit", browserHeaders)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "Index", pageData(t, body).Kind)
	})

	t.Run("PlainGetServesFile", func(t *testing.T) {
		resp, body := doRequest(t, "GET", url+"note.txt", browserHeaders)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "ciao", body)
	})

	t.Run("TooBig", func(t *testing.T) {
		url := startWebUIDir(t, dir, nil, func(o *webui.Options) { o.EditMaxSize = 2 }, nil)
		_, body := doRequest(t, "GET", url+"note.txt?edit", nil)
		assert.False(t, editData(t, body).Editable)
	})

	t.Run("ReadOnlyAndUser", func(t *testing.T) {
		vfsOpt := vfscommon.Opt
		vfsOpt.ReadOnly = true
		url := startWebUIDir(t, dir, func(opt *Options) {
			opt.Auth.BasicUser = testUser
			opt.Auth.BasicPass = testPass
		}, nil, &vfsOpt)
		_, body := doRequest(t, "GET", url+"note.txt?edit", map[string]string{
			"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(testUser+":"+testPass)),
		})
		data := editData(t, body)
		assert.False(t, data.AllowUpload)
		assert.False(t, data.AllowDelete)
		assert.True(t, data.Auth)
		require.NotNil(t, data.User)
		assert.Equal(t, testUser, *data.User)
	})

	t.Run("BaseURL", func(t *testing.T) {
		url := startWebUIDir(t, dir, func(opt *Options) { opt.HTTP.BaseURL = "/pfx" }, nil, nil)
		_, body := doRequest(t, "GET", url+"note.txt?edit", nil)
		data := editData(t, body)
		assert.Equal(t, "/note.txt", data.Href)
		assert.Equal(t, "/pfx/", data.URIPrefix)
	})
}

// TestFileCSPSandbox checks the files which can run scripts are sandboxed
func TestFileCSPSandbox(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"x.html": "<script>alert(1)</script>",
		"y.SVG":  "<svg/>",
		"x.txt":  "text",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0666))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, "d.html"), 0777))
	url := startWebUIDir(t, dir, nil, nil, nil)

	resp, body := doRequest(t, "GET", url+"x.html", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "<script>alert(1)</script>", body)

	// Also for a browser, which the listing looks at first
	resp, body = doRequest(t, "GET", url+"x.html", browserHeaders)
	assert.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "<script>alert(1)</script>", body)

	resp, _ = doRequest(t, "HEAD", url+"x.html", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))

	resp, _ = doRequest(t, "GET", url+"y.SVG", nil)
	assert.Equal(t, "sandbox", resp.Header.Get("Content-Security-Policy"))

	resp, body = doRequest(t, "GET", url+"x.txt", nil)
	assert.Equal(t, "text", body)
	assert.Empty(t, resp.Header.Get("Content-Security-Policy"))

	// The edit page is not sandboxed as a file would be
	resp, _ = doRequest(t, "GET", url+"x.html?edit", nil)
	assert.NotEqual(t, "sandbox", resp.Header.Get("Content-Security-Policy"))

	// A directory is still redirected and listed
	resp, _ = doRequest(t, "GET", url+"d.html", browserHeaders)
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/d.html/", resp.Header.Get("Location"))
	resp, body = doRequest(t, "GET", url+"d.html/", browserHeaders)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Index", pageData(t, body).Kind)
	assert.NotEqual(t, "sandbox", resp.Header.Get("Content-Security-Policy"))

	t.Run("Disabled", func(t *testing.T) {
		url := startWebUIDir(t, dir, nil, func(o *webui.Options) { o.Enabled = false }, nil)
		resp, body := doRequest(t, "GET", url+"x.html", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "<script>alert(1)</script>", body)
		assert.Empty(t, resp.Header.Get("Content-Security-Policy"))
	})
}

// resumeRecord decodes the body of a ?resume-get reply
type resumeRecord struct {
	Position  float64  `json:"position"`
	Duration  *float64 `json:"duration"`
	Timestamp int64    `json:"timestamp"`
	FilePath  string   `json:"file_path"`
}

// TestWebUIResumeEndpoints checks the playback resume API and the
// positions in the listing
func TestWebUIResumeEndpoints(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0777))
	for _, name := range []string{"v.mp4", "x.txt", "sub/a.MP3", "sub/b.mkv"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("data"), 0666))
	}
	resumeDir := t.TempDir()
	enable := func(o *webui.Options) {
		o.Resume = true
		o.ResumeDir = resumeDir
	}
	positions := func(t *testing.T, body string) map[string]*int64 {
		t.Helper()
		out := map[string]*int64{}
		for _, item := range jsonData(t, body).Paths {
			out[item.Name] = item.ResumePosition
		}
		return out
	}

	t.Run("Enabled", func(t *testing.T) {
		url := startWebUIDir(t, dir, nil, enable, nil)

		resp, body := doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		assert.Equal(t, `{"error":"No resume position found"}`, body)

		_, body = doRequest(t, "GET", url+"?json", nil)
		data := jsonData(t, body)
		assert.True(t, data.PlaybackResumeEnabled)
		assert.Equal(t, map[string]*int64{"sub": nil, "v.mp4": nil, "x.txt": nil}, positions(t, body))

		resp, body = doRequest(t, "GET", url+"v.mp4?resume-save=12.5&duration=100", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Empty(t, body)

		resp, body = doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		var rec resumeRecord
		require.NoError(t, json.Unmarshal([]byte(body), &rec))
		assert.Equal(t, 12.5, rec.Position)
		require.NotNil(t, rec.Duration)
		assert.Equal(t, 100.0, *rec.Duration)
		assert.Equal(t, "/v.mp4", rec.FilePath)
		assert.InDelta(t, time.Now().Unix(), rec.Timestamp, 60)

		resp, body = doRequest(t, "HEAD", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, body)

		_, body = doRequest(t, "GET", url+"?json", nil)
		got := positions(t, body)
		require.NotNil(t, got["v.mp4"])
		assert.Equal(t, int64(12), *got["v.mp4"], "the position is truncated")
		assert.Nil(t, got["x.txt"])

		// The page embeds the same data
		_, body = doRequest(t, "GET", url, browserHeaders)
		assert.True(t, pageData(t, body).PlaybackResumeEnabled)

		// Positions are for a path: another file is not affected
		resp, _ = doRequest(t, "GET", url+"sub/b.mkv?resume-get", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		// Saves in a subdirectory with an upper case extension show in its listing
		resp, _ = doRequest(t, "GET", url+"sub/a.MP3?resume-save=3&duration=NaN", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp, body = doRequest(t, "GET", url+"sub/a.MP3?resume-get", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		rec = resumeRecord{}
		require.NoError(t, json.Unmarshal([]byte(body), &rec))
		assert.Nil(t, rec.Duration, "a duration which isn't finite is null")
		_, body = doRequest(t, "GET", url+"sub/?json", nil)
		got = positions(t, body)
		require.NotNil(t, got["a.MP3"])
		assert.Equal(t, int64(3), *got["a.MP3"])
		assert.Nil(t, got["b.mkv"])

		// Invalid requests
		for _, query := range []string{"-1", "abc", "", "NaN", "Inf", "-Inf"} {
			resp, _ = doRequest(t, "GET", url+"v.mp4?resume-save="+query, nil)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, query)
		}
		resp, _ = doRequest(t, "GET", url+"x.txt?resume-save=1", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "not media")
		resp, _ = doRequest(t, "GET", url+"missing.mp4?resume-save=1", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "doesn't exist")
		resp, _ = doRequest(t, "GET", url+"sub?resume-save=1", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "directory")
		entries, err := os.ReadDir(resumeDir)
		require.NoError(t, err)
		assert.Len(t, entries, 2, "failed saves leave nothing in the store")

		// Reading and clearing need no file
		resp, _ = doRequest(t, "GET", url+"gone.mp4?resume-get", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		resp, _ = doRequest(t, "GET", url+"gone.mp4?resume-clear", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		resp, _ = doRequest(t, "GET", url+"v.mp4?resume-clear", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp, body = doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, `{"error":"No resume position found"}`, body)

		// The file itself is untouched
		_, body = doRequest(t, "GET", url+"v.mp4", nil)
		assert.Equal(t, "data", body)
	})

	t.Run("SharedByUsers", func(t *testing.T) {
		url := startWebUIDir(t, dir, func(opt *Options) {
			opt.Auth.BasicUser = testUser
			opt.Auth.BasicPass = testPass
		}, enable, nil)
		resp, _ := doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "needs authentication")
		auth := map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(testUser+":"+testPass))}
		resp, _ = doRequest(t, "GET", url+"v.mp4?resume-save=7", auth)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		_, body := doRequest(t, "GET", url+"?json", auth)
		got := positions(t, body)
		require.NotNil(t, got["v.mp4"])
		assert.Equal(t, int64(7), *got["v.mp4"])
	})

	t.Run("ReadOnly", func(t *testing.T) {
		vfsOpt := vfscommon.Opt
		vfsOpt.ReadOnly = true
		url := startWebUIDir(t, dir, nil, enable, &vfsOpt)
		resp, _ := doRequest(t, "GET", url+"v.mp4?resume-save=9", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, "positions are not written to the remote")
	})

	t.Run("BaseURL", func(t *testing.T) {
		url := startWebUIDir(t, dir, func(opt *Options) { opt.HTTP.BaseURL = "/pfx" }, enable, nil)
		resp, _ := doRequest(t, "GET", url+"sub/b.mkv?resume-save=4", nil)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp, body := doRequest(t, "GET", url+"sub/b.mkv?resume-get", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var rec resumeRecord
		require.NoError(t, json.Unmarshal([]byte(body), &rec))
		assert.Equal(t, "/sub/b.mkv", rec.FilePath, "without the base URL")
	})

	t.Run("Disabled", func(t *testing.T) {
		url := startWebUIDir(t, dir, nil, func(o *webui.Options) {
			o.Resume = false
			o.ResumeDir = resumeDir
		}, nil)
		resp, body := doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, `{"error":"Playback resume disabled"}`, body)
		resp, body = doRequest(t, "GET", url+"v.mp4?resume-save=1", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, `{"error":"Playback resume disabled"}`, body)
		resp, _ = doRequest(t, "GET", url+"v.mp4?resume-clear", nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		_, body = doRequest(t, "GET", url+"?json", nil)
		data := jsonData(t, body)
		assert.False(t, data.PlaybackResumeEnabled)
		assert.Equal(t, map[string]*int64{"sub": nil, "v.mp4": nil, "x.txt": nil}, positions(t, body), "no positions are read")
	})

	t.Run("UIDisabled", func(t *testing.T) {
		url := startWebUIDir(t, dir, nil, func(o *webui.Options) {
			o.Enabled = false
			o.Resume = true
			o.ResumeDir = resumeDir
		}, nil)
		resp, _ := doRequest(t, "GET", url+"v.mp4?resume-get", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode, "the file, as for upstream")
		assert.Empty(t, resp.Header.Get("Cache-Control"))
	})
}

// zipEntries returns the contents of the zip in body by entry name
func zipEntries(t *testing.T, body string) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader([]byte(body)), int64(len(body)))
	require.NoError(t, err)
	entries := map[string]string{}
	for _, file := range zr.File {
		rc, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		entries[file.Name] = string(data)
	}
	return entries
}

// TestWebUIZip checks the zip downloads of directories and selections
func TestWebUIZip(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"a.txt":         "aaa",
		"d/b.txt":       "bbbb",
		"d/sub/s.txt":   "s",
		"outside/o.txt": "o",
	} {
		full := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0777))
		require.NoError(t, os.WriteFile(full, []byte(content), 0666))
	}
	base := startWebUIDir(t, dir, nil, nil, nil)
	multi := func(paths string) string {
		return "?zip=multi&paths=" + url.QueryEscape(paths)
	}

	t.Run("Directory", func(t *testing.T) {
		resp, body := doRequest(t, "GET", base+"d/?zip", nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
		assert.Equal(t, `attachment; filename=d.zip`, resp.Header.Get("Content-Disposition"))
		assert.NotEmpty(t, resp.Header.Get("Last-Modified"))
		entries := zipEntries(t, body)
		assert.Equal(t, "bbbb", entries["b.txt"])
		assert.Equal(t, "s", entries["sub/s.txt"])
		assert.Contains(t, entries, "sub/")
		assert.NotContains(t, entries, "a.txt")
	})

	t.Run("Root", func(t *testing.T) {
		resp, body := doRequest(t, "GET", base+"?zip", nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, `attachment; filename=root.zip`, resp.Header.Get("Content-Disposition"))
		entries := zipEntries(t, body)
		assert.Equal(t, "aaa", entries["a.txt"])
		assert.Equal(t, "bbbb", entries["d/b.txt"])
		assert.Equal(t, "s", entries["d/sub/s.txt"])
		assert.Equal(t, "o", entries["outside/o.txt"])
	})

	t.Run("ForCommandLineClient", func(t *testing.T) {
		resp, body := doRequest(t, "GET", base+"d/?zip=", map[string]string{"User-Agent": "curl/8"})
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
		assert.Contains(t, zipEntries(t, body), "b.txt")
	})

	t.Run("Selection", func(t *testing.T) {
		resp, body := doRequest(t, "GET", base+multi(`["a.txt","d"]`), nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
		assert.Equal(t, `attachment; filename=selection.zip`, resp.Header.Get("Content-Disposition"))
		entries := zipEntries(t, body)
		assert.Equal(t, "aaa", entries["a.txt"])
		assert.Equal(t, "bbbb", entries["d/b.txt"])
		assert.Equal(t, "s", entries["d/sub/s.txt"])
		assert.Contains(t, entries, "d/")
		assert.NotContains(t, entries, "outside/o.txt")
	})

	t.Run("SelectionInSubdirectory", func(t *testing.T) {
		_, body := doRequest(t, "GET", base+"d/"+multi(`["sub","b.txt"]`), nil)
		entries := zipEntries(t, body)
		assert.Equal(t, "bbbb", entries["b.txt"])
		assert.Equal(t, "s", entries["sub/s.txt"])
		assert.NotContains(t, entries, "a.txt")
	})

	t.Run("Head", func(t *testing.T) {
		for _, query := range []string{"d/?zip", multi(`["a.txt","d"]`)} {
			resp, body := doRequest(t, "HEAD", base+query, nil)
			assert.Equal(t, 200, resp.StatusCode, query)
			assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"), query)
			assert.NotEmpty(t, resp.Header.Get("Content-Disposition"), query)
			assert.Empty(t, body, query)
		}
	})

	t.Run("HeadValidates", func(t *testing.T) {
		resp, _ := doRequest(t, "HEAD", base+multi(`["nope"]`), nil)
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("Rejected", func(t *testing.T) {
		for _, test := range []struct {
			name   string
			query  string
			status int
			msg    string
		}{
			{"Missing", "?zip=multi", 400, "Missing paths parameter"},
			{"NotJSON", "?zip=multi&paths=a.txt", 400, "Invalid selection payload"},
			{"Empty", multi(`[]`), 400, "Selection is empty"},
			{"Parent", multi(`["../x"]`), 400, "Invalid selection path"},
			{"ParentInside", multi(`["d/../a.txt"]`), 400, "Invalid selection path"},
			{"EncodedParent", "?zip=multi&paths=%5B%22%2e%2e%2fx%22%5D", 400, "Invalid selection path"},
			{"Backslash", multi(`["d\\b.txt"]`), 400, "Invalid selection path"},
			// Leading slashes are trimmed so it is relative to the directory
			{"Absolute", "d/" + multi(`["/outside/o.txt"]`), 404, ""},
			{"Nonexistent", multi(`["nope"]`), 404, ""},
			{"NonexistentBelowFile", multi(`["a.txt/x"]`), 404, ""},
		} {
			t.Run(test.name, func(t *testing.T) {
				resp, body := doRequest(t, "GET", base+test.query, nil)
				assert.Equal(t, test.status, resp.StatusCode)
				if test.msg != "" {
					assert.Equal(t, test.msg, strings.TrimSpace(body))
				}
			})
		}
	})

	t.Run("NotADirectory", func(t *testing.T) {
		// Without the slash it is a file for the upstream handler
		resp, body := doRequest(t, "GET", base+"a.txt?zip", nil)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "aaa", body)
	})

	t.Run("DirectoryWithoutSlashRedirects", func(t *testing.T) {
		resp, _ := doRequest(t, "GET", base+"d?zip=multi", browserHeaders)
		assert.Equal(t, 302, resp.StatusCode)
		assert.Equal(t, "/d/?zip=multi", resp.Header.Get("Location"))
	})

	t.Run("MultiZipMax", func(t *testing.T) {
		base := startWebUIDir(t, dir, nil, func(opt *webui.Options) { opt.MultiZipMax = 1 }, nil)
		resp, body := doRequest(t, "GET", base+multi(`["a.txt","d"]`), nil)
		assert.Equal(t, 400, resp.StatusCode)
		assert.Equal(t, "Too many selected paths", strings.TrimSpace(body))
		resp, _ = doRequest(t, "GET", base+multi(`["a.txt"]`), nil)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("DisableZip", func(t *testing.T) {
		base := startWebUIDir(t, dir, func(opt *Options) { opt.DisableZip = true }, nil, nil)
		for _, query := range []string{"?zip", "d/?zip", multi(`["a.txt"]`)} {
			resp, body := doRequest(t, "GET", base+query, nil)
			assert.Equal(t, 403, resp.StatusCode, query)
			assert.Equal(t, "Zip downloads are disabled", strings.TrimSpace(body), query)
		}
		_, body := doRequest(t, "GET", base+"?json", nil)
		assert.False(t, jsonData(t, body).AllowArchive)
	})
}
