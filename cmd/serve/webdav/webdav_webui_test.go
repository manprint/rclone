// fork:webui

//go:build !windows && !darwin

package webdav

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
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
	savedOpt := webui.Opt
	t.Cleanup(func() { webui.Opt = savedOpt })
	webui.Opt.Enabled = true
	if uiMutate != nil {
		uiMutate(&webui.Opt)
	}

	f, err := fs.NewFs(context.Background(), t.TempDir())
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
