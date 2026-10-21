package webui

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreAuthHoldsVFS checks the downloads served before authentication run
// with the VFS held, and the assets and the requests left to the next
// handler don't
func TestPreAuthHoldsVFS(t *testing.T) {
	VFS, _ := newTestVFS(t, map[string]string{"pub/a.txt": "hello"})
	opt := Options{TokenExpiry: fs.Duration(time.Hour), PublinkPaths: fs.CommaSepList{"pub"}}
	newUI := func(hold func(http.Handler) http.Handler) *UI {
		return newTokenUI(t, opt, func(c *Config) {
			c.GetVFS = func(context.Context) (*vfs.VFS, error) { return VFS, nil }
			c.HoldVFS = hold
		})
	}

	t.Run("Held", func(t *testing.T) {
		held := 0
		u := newUI(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				held++
				next.ServeHTTP(w, r)
			})
		})
		next := &fallthroughHandler{}
		h := u.PreAuth(next)

		tok := u.makeToken("alice", "pub/a.txt", time.Now())
		rec := do(h, "GET", "/pub/a.txt?token="+tok)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "hello", rec.Body.String())
		assert.Equal(t, 1, held, "token download")

		rec = do(h, "GET", "/@public/"+u.pubToken("pub")+"/pub/a.txt")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "hello", rec.Body.String())
		assert.Equal(t, 2, held, "public link")

		rec = do(h, "GET", "/pub/a.txt?token=bad")
		assert.Equal(t, http.StatusTeapot, rec.Code)
		assert.Equal(t, []string{"/pub/a.txt"}, next.paths)
		assert.Equal(t, 2, held, "bad token")

		rec = do(h, "GET", "/"+assetsPrefix+"index.js")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 2, held, "asset")
	})

	t.Run("NoHoldVFS", func(t *testing.T) {
		u := newUI(nil)
		h := u.PreAuth(&fallthroughHandler{})
		tok := u.makeToken("alice", "pub/a.txt", time.Now())
		rec := do(h, "GET", "/pub/a.txt?token="+tok)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "hello", rec.Body.String())
		rec = do(h, "GET", "/@public/"+u.pubToken("pub")+"/pub/a.txt")
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
