package webui

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
)

// newPublicUI returns a UI for the tests of public links serving the
// files of content, with the public link paths
func newPublicUI(t *testing.T, paths []string, content map[string]string, mutate func(*Config)) *UI {
	t.Helper()
	VFS, _ := newTestVFS(t, content)
	return newTokenUI(t, Options{PublinkPaths: fs.CommaSepList(paths)}, func(c *Config) {
		c.GetVFS = func(context.Context) (*vfs.VFS, error) { return VFS, nil }
		if mutate != nil {
			mutate(c)
		}
	})
}

// TestPublinkAllowed checks which remotes the paths for public links allow
func TestPublinkAllowed(t *testing.T) {
	for _, test := range []struct {
		name   string
		paths  []string
		remote string
		want   bool
	}{
		{"NoPaths", nil, "pub", false},
		{"NoPathsRoot", nil, "", false},
		{"Exact", []string{"pub"}, "pub", true},
		{"Below", []string{"pub"}, "pub/x", true},
		{"BelowDeep", []string{"pub"}, "pub/x/y/z.txt", true},
		{"SharedPrefix", []string{"pub"}, "pubx", false},
		{"SharedPrefixBelow", []string{"pub"}, "pubx/f", false},
		{"Parent", []string{"a/b"}, "a", false},
		{"Other", []string{"pub"}, "priv/f", false},
		{"Root", []string{"pub"}, "", false},
		{"Slashes", []string{"/a/b/"}, "a/b", true},
		{"SlashesBelow", []string{"/a/b/"}, "a/b/c", true},
		{"SlashesSibling", []string{"/a/b/"}, "a/bc", false},
		{"SlashesParent", []string{"/a/b/"}, "a", false},
		{"RootEntryFile", []string{"/"}, "f.txt", true},
		{"RootEntryDeep", []string{"/"}, "a/b/c", true},
		{"RootEntryRoot", []string{"/"}, "", true},
		{"Second", []string{"pub", "/a/b/"}, "a/b/c", true},
		{"SecondOther", []string{"pub", "/a/b/"}, "a/c", false},
		{"EmptyEntry", []string{""}, "x", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			u := newTokenUI(t, Options{PublinkPaths: fs.CommaSepList(test.paths)}, nil)
			assert.Equal(t, test.want, u.allowed(test.remote))
		})
	}
}

// TestPubTokenSubtree checks a public link token is valid for the remote
// it was made for and everything below it, and nothing else
func TestPubTokenSubtree(t *testing.T) {
	u := newTokenUI(t, Options{Secret: "one"}, nil)
	tok := u.pubToken("pub")
	assert.Len(t, tok, 32)
	assert.Equal(t, strings.ToLower(tok), tok)

	for _, test := range []struct {
		name   string
		tok    string
		remote string
		want   bool
	}{
		{"Same", tok, "pub", true},
		{"Child", tok, "pub/f", true},
		{"Descendant", tok, "pub/sub/f", true},
		{"DescendantDir", tok, "pub/sub", true},
		{"Other", tok, "priv/f", false},
		{"SharedPrefix", tok, "pubx/f", false},
		{"SharedPrefixDir", tok, "pubx", false},
		{"Parent", tok, "", false},
		{"Sibling", tok, "pub2", false},
		{"OtherName", u.pubToken("priv"), "pub/f", false},
		{"FileToOtherFile", u.pubToken("pub/a.txt"), "pub/b.txt", false},
		{"FileToItself", u.pubToken("pub/a.txt"), "pub/a.txt", true},
		{"FileToChild", u.pubToken("pub/a.txt"), "pub/a.txt/x", true},
		{"FileToDir", u.pubToken("pub/a.txt"), "pub", false},
		{"Root", u.pubToken(""), "", true},
		{"RootCoversAll", u.pubToken(""), "pub/sub/f", true},
		{"Empty", "", "pub/f", false},
		{"Short", tok[:31], "pub/f", false},
		{"Long", tok + "0", "pub/f", false},
		{"Uppercase", strings.ToUpper(tok), "pub/f", false},
		{"Garbage", "bad", "pub/f", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, u.checkPubToken(test.tok, test.remote))
		})
	}

	t.Run("OtherSecret", func(t *testing.T) {
		other := newTokenUI(t, Options{Secret: "two"}, nil)
		assert.NotEqual(t, tok, other.pubToken("pub"))
		assert.False(t, other.checkPubToken(tok, "pub/f"))
	})
	t.Run("SameSecret", func(t *testing.T) {
		same := newTokenUI(t, Options{Secret: "one"}, nil)
		assert.Equal(t, tok, same.pubToken("pub"))
		assert.True(t, same.checkPubToken(tok, "pub/f"))
	})
	t.Run("NotDownloadKey", func(t *testing.T) {
		// A download token can't be passed off as a public link
		assert.NotEqual(t, u.keys.token, u.keys.publink)
		assert.False(t, u.checkPubToken(u.makeToken("", "pub", time.Now()), "pub"))
	})
}

// TestPublicIgnoredInProxy checks public links aren't served with an auth
// proxy, or when no paths are set, and are otherwise
func TestPublicIgnoredInProxy(t *testing.T) {
	content := map[string]string{"pub/a.txt": "aaa"}
	for _, test := range []struct {
		name     string
		paths    []string
		proxy    bool
		method   string
		wantNext bool
	}{
		{"Served", []string{"pub"}, false, "GET", false},
		{"ServedHead", []string{"pub"}, false, "HEAD", false},
		{"Proxy", []string{"pub"}, true, "GET", true},
		{"ProxyHead", []string{"pub"}, true, "HEAD", true},
		{"NoPaths", nil, false, "GET", true},
		{"NoPathsProxy", nil, true, "GET", true},
		{"Put", []string{"pub"}, false, "PUT", true},
		{"Post", []string{"pub"}, false, "POST", true},
		{"Delete", []string{"pub"}, false, "DELETE", true},
		{"Propfind", []string{"pub"}, false, "PROPFIND", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			u := newPublicUI(t, test.paths, content, func(c *Config) { c.IsProxy = test.proxy })
			// The token can only be made by a server which makes them
			tok := u.pubToken("pub")
			next := &fallthroughHandler{}
			target := "/@public/" + tok + "/pub/a.txt"
			rec := do(u.PreAuth(next), test.method, target)
			if test.wantNext {
				assert.Equal(t, http.StatusTeapot, rec.Code)
				assert.Equal(t, []string{"/@public/" + tok + "/pub/a.txt"}, next.paths)
				return
			}
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, next.paths)
			if test.method == "GET" {
				assert.Equal(t, "aaa", rec.Body.String())
			}
		})
	}
}

// TestPublicForbiddenWhenNotAllowed checks a valid token for something
// outside the paths now set isn't served
func TestPublicForbiddenWhenNotAllowed(t *testing.T) {
	content := map[string]string{"old/f.txt": "old", "new/g.txt": "new"}
	u := newPublicUI(t, []string{"new"}, content, nil)
	h := u.PreAuth(&fallthroughHandler{})

	oldTok := u.pubToken("old")
	rootTok := u.pubToken("")
	for _, target := range []string{
		"/@public/" + oldTok + "/old",
		"/@public/" + oldTok + "/old/",
		"/@public/" + oldTok + "/old/?json",
		"/@public/" + oldTok + "/old/?zip",
		"/@public/" + oldTok + "/old/f.txt",
		"/@public/" + oldTok + "/old/f.txt?view",
		"/@public/" + rootTok + "/old/f.txt",
		"/@public/" + rootTok + "/",
	} {
		rec := do(h, "GET", target)
		assert.Equal(t, http.StatusForbidden, rec.Code, target)
		assert.NotContains(t, rec.Body.String(), "old", target)
	}

	// What the paths allow is served
	newTok := u.pubToken("new")
	rec := do(h, "GET", "/@public/"+newTok+"/new/g.txt")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "new", rec.Body.String())
	rec = do(h, "GET", "/@public/"+rootTok+"/new/g.txt")
	assert.Equal(t, http.StatusOK, rec.Code)

	// Something which doesn't exist is a 404 whether allowed or not
	assert.Equal(t, http.StatusNotFound, do(h, "GET", "/@public/"+oldTok+"/old/missing").Code)
	assert.Equal(t, http.StatusNotFound, do(h, "GET", "/@public/"+newTok+"/new/missing").Code)
}
