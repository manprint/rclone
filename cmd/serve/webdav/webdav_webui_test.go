// fork:webui

//go:build !windows && !darwin

package webdav

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
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
