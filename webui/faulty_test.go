package webui

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errUnreachable is a retriable error as a backend returns when its remote
// can't be reached
var errUnreachable = fserrors.RetryErrorf("remote unreachable")

// faultyFs is an Fs whose listings, object lookups and uploads fail or
// hang on demand, standing in for a remote which can't be reached
type faultyFs struct {
	fs.Fs
	mu        sync.Mutex
	listFault func(dir string) error // run before each listing, which fails with its error
	objectErr error                  // returned by NewObject if set
	putErr    error                  // returned by Put if set
	lists     atomic.Int32           // number of listings
}

// setListFault sets the function run before each listing
func (f *faultyFs) setListFault(fn func(dir string) error) {
	f.mu.Lock()
	f.listFault = fn
	f.mu.Unlock()
}

// failLists makes the listings of the directories dirs, or all of them if
// none are given, fail with err
func (f *faultyFs) failLists(err error, dirs ...string) {
	f.setListFault(func(dir string) error {
		if len(dirs) == 0 {
			return err
		}
		for _, d := range dirs {
			if d == dir {
				return err
			}
		}
		return nil
	})
}

// hangLists makes the listings of the directories dirs wait until the test
// ends
func (f *faultyFs) hangLists(t *testing.T, dirs ...string) {
	release := make(chan struct{})
	// Registered after the VFS so it runs before VFS.Shutdown
	t.Cleanup(func() { close(release) })
	f.setListFault(func(dir string) error {
		for _, d := range dirs {
			if d == dir {
				<-release
			}
		}
		return nil
	})
}

func (f *faultyFs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	f.lists.Add(1)
	f.mu.Lock()
	fault := f.listFault
	f.mu.Unlock()
	if fault != nil {
		if err := fault(dir); err != nil {
			return nil, err
		}
	}
	return f.Fs.List(ctx, dir)
}

func (f *faultyFs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	f.mu.Lock()
	err := f.objectErr
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.Fs.NewObject(ctx, remote)
}

func (f *faultyFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	f.mu.Lock()
	err := f.putErr
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return f.Fs.Put(ctx, in, src, options...)
}

// Features hides the optional methods which would reach the wrapped Fs
// without going through the faults
func (f *faultyFs) Features() *fs.Features {
	ft := *f.Fs.Features()
	ft.ListP = nil
	ft.ListR = nil
	ft.PutStream = nil
	return &ft
}

// newFaultyVFS is newTestVFS on a faultyFs
func newFaultyVFS(t *testing.T, content map[string]string) (*vfs.VFS, *faultyFs, string) {
	t.Helper()
	dir := t.TempDir()
	for name, data := range content {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			require.NoError(t, os.MkdirAll(full, 0777))
			continue
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0777))
		require.NoError(t, os.WriteFile(full, []byte(data), 0666))
	}
	f, err := fs.NewFs(context.Background(), dir)
	require.NoError(t, err)
	ff := &faultyFs{Fs: f}
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeOff
	VFS := vfs.New(context.Background(), ff, &opt)
	t.Cleanup(VFS.Shutdown)
	return VFS, ff, dir
}

// newFaultyUI returns a UI serving VFS with the options opt
func newFaultyUI(t *testing.T, VFS *vfs.VFS, opt Options) *UI {
	t.Helper()
	return newTokenUI(t, opt, func(c *Config) {
		c.GetVFS = func(context.Context) (*vfs.VFS, error) { return VFS, nil }
	})
}

// doWithin is do which fails the test instead of hanging if the request
// takes longer than limit
func doWithin(t *testing.T, limit time.Duration, h http.Handler, method, target string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- do(h, method, target, headers...) }()
	select {
	case rec := <-done:
		return rec
	case <-time.After(limit):
		t.Fatalf("%s %s still running after %v", method, target, limit)
		return nil
	}
}

const browserAccept = "text/html,application/xhtml+xml"

// TestErrStatusRemote checks the HTTP status of the errors of a remote
// which can't be reached
func TestErrStatusRemote(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{"Deadline", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"WrappedDeadline", fmt.Errorf("list: %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		{"Canceled", context.Canceled, http.StatusServiceUnavailable},
		{"Retriable", errUnreachable, http.StatusServiceUnavailable},
		{"Refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, http.StatusServiceUnavailable},
		{"Other", errors.New("boom"), http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, errStatus(test.err))
		})
	}
}

// TestRefuseDeleteFailsClosed checks --webui-allow-delete=false refuses
// the requests it can't check because the remote fails, and lets through
// those onto paths the remote says don't exist
func TestRefuseDeleteFailsClosed(t *testing.T) {
	opt := Options{AllowDelete: false}

	t.Run("ListingFails", func(t *testing.T) {
		VFS, f, _ := newFaultyVFS(t, map[string]string{"a.txt": "old"})
		f.failLists(errUnreachable)
		f.objectErr = errUnreachable
		next := &fallthroughHandler{}
		h := newFaultyUI(t, VFS, opt).Handler(next)

		rec := do(h, "PUT", "/a.txt")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		rec = do(h, "COPY", "/b.txt", "Destination", "/a.txt")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Empty(t, next.paths)
	})

	t.Run("StaleCacheLookupFails", func(t *testing.T) {
		VFS, f, dir := newFaultyVFS(t, map[string]string{"d/": ""})
		// The directory is cached empty, then another client adds a file
		_, err := VFS.Stat("d/a.txt")
		require.ErrorIs(t, err, vfs.ENOENT)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "d", "a.txt"), []byte("old"), 0666))
		f.objectErr = errUnreachable
		next := &fallthroughHandler{}
		h := newFaultyUI(t, VFS, opt).Handler(next)

		rec := do(h, "PUT", "/d/a.txt")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Empty(t, next.paths)
	})

	t.Run("NotFoundPassesThrough", func(t *testing.T) {
		for _, notFound := range []error{fs.ErrorObjectNotFound, fs.ErrorDirNotFound} {
			VFS, f, _ := newFaultyVFS(t, map[string]string{"d/": ""})
			f.objectErr = notFound
			next := &fallthroughHandler{}
			h := newFaultyUI(t, VFS, opt).Handler(next)

			rec := do(h, "PUT", "/d/new.txt")
			assert.Equal(t, http.StatusTeapot, rec.Code, "%v", notFound)
		}
	})

	t.Run("GuardMove", func(t *testing.T) {
		VFS, f, _ := newFaultyVFS(t, map[string]string{"a.txt": "a", "d/": ""})
		f.failLists(errUnreachable)
		next := &fallthroughHandler{}
		h := newFaultyUI(t, VFS, Options{AllowDelete: true}).Handler(next)

		rec := do(h, "MOVE", "/a.txt", "Origin", "http://example.com", "Destination", "/d")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Empty(t, next.paths)
	})
}

// TestZipAbortsOnFailure checks a zip which fails once its body has
// started ends with a broken connection rather than a complete response
func TestZipAbortsOnFailure(t *testing.T) {
	// The file doesn't compress and is big enough for the headers and the
	// start of the zip to be sent before the failure
	data := make([]byte, 1<<20)
	_, err := rand.Read(data)
	require.NoError(t, err)
	VFS, f, _ := newFaultyVFS(t, map[string]string{"a.bin": string(data), "d/b.txt": "bbb"})
	f.failLists(errUnreachable, "d")
	srv := httptest.NewServer(newFaultyUI(t, VFS, Options{MultiZipMax: 10}).Handler(&fallthroughHandler{}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/?zip=multi&paths=" + url.QueryEscape(`["a.bin","d"]`))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_, err = io.ReadAll(resp.Body)
	assert.Error(t, err, "the client must see the zip as failed")
}

// TestNoFallthroughOnRemoteError checks the requests of the UI reply with
// the error of the remote instead of passing to the next handler, which
// would repeat the failing lookup
func TestNoFallthroughOnRemoteError(t *testing.T) {
	VFS, f, _ := newFaultyVFS(t, map[string]string{"pub/a.mp4": "a", "d/": ""})
	f.failLists(errUnreachable)
	opt := Options{TokenExpiry: fs.Duration(time.Hour), PublinkPaths: fs.CommaSepList{"pub"}, Resume: true, ResumeDir: t.TempDir()}
	u := newFaultyUI(t, VFS, opt)
	next := &fallthroughHandler{}
	h := u.Handler(next)
	pre := u.PreAuth(next)

	for _, test := range []struct {
		name   string
		h      http.Handler
		target string
	}{
		{"Listing", h, "/d/"},
		{"ListingJSON", h, "/d/?json"},
		{"Edit", h, "/pub/a.mp4?edit"},
		{"View", h, "/pub/a.mp4?view"},
		{"Tokengen", h, "/pub/a.mp4?tokengen"},
		{"ResumeSave", h, "/pub/a.mp4?resume-save=3"},
		{"Public", pre, "/@public/" + u.pubToken("pub") + "/pub/a.mp4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := do(test.h, "GET", test.target, "Accept", browserAccept)
			assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		})
	}
	assert.Empty(t, next.paths)

	t.Run("NotFoundFallsThrough", func(t *testing.T) {
		f.setListFault(nil)
		rec := do(h, "GET", "/nope/", "Accept", browserAccept)
		assert.Equal(t, http.StatusTeapot, rec.Code)
	})
}

// TestUploadBackendFailure checks an upload which the remote refuses is
// reported as a failure of the server rather than of the client
func TestUploadBackendFailure(t *testing.T) {
	VFS, f, _ := newFaultyVFS(t, map[string]string{"d/": ""})
	f.putErr = errUnreachable
	h := newFaultyUI(t, VFS, Options{AllowDelete: true}).Handler(&fallthroughHandler{})

	req := httptest.NewRequest("PUT", "/d/a.txt", strings.NewReader("hello"))
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	t.Run("ShortBody", func(t *testing.T) {
		f.putErr = nil
		req := httptest.NewRequest("PUT", "/d/b.txt", strings.NewReader("hel"))
		req.ContentLength = 5
		req.Header.Set("Origin", "http://example.com")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// TestHungRemoteSharedLookup checks the requests waiting on a remote which
// hangs give up with their contexts and share a single listing
func TestHungRemoteSharedLookup(t *testing.T) {
	VFS, f, _ := newFaultyVFS(t, map[string]string{"d/a.txt": "a"})
	f.hangLists(t, "d")
	h := newFaultyUI(t, VFS, Options{}).Handler(&fallthroughHandler{})

	const n = 5
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			req := httptest.NewRequest("GET", "/d/?json", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("requests still waiting on the hung remote")
	}
	for _, code := range codes {
		assert.Equal(t, http.StatusGatewayTimeout, code)
	}
	assert.Equal(t, int32(2), f.lists.Load(), "the root and one listing of d shared by all the requests")
}

// TestSearchBoundedByTimeout checks a search returns within its timeout
// when a directory hangs, and says its results are incomplete
func TestSearchBoundedByTimeout(t *testing.T) {
	VFS, f, _ := newFaultyVFS(t, map[string]string{"match1.txt": "1", "x/match2.txt": "2", "y/match3.txt": "3"})
	opt := Options{Search: true, SearchMaxDepth: 8, SearchMaxResults: 100, SearchTimeout: fs.Duration(200 * time.Millisecond)}
	h := newFaultyUI(t, VFS, opt).Handler(&fallthroughHandler{})

	decode := func(rec *httptest.ResponseRecorder) (names []string, incomplete bool) {
		require.Equal(t, http.StatusOK, rec.Code)
		var data struct {
			Paths      []PathItem `json:"paths"`
			Incomplete bool       `json:"search_incomplete"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &data))
		for _, p := range data.Paths {
			names = append(names, p.Name)
		}
		return names, data.Incomplete
	}

	t.Run("Failing", func(t *testing.T) {
		f.failLists(errUnreachable, "x")
		names, incomplete := decode(doWithin(t, 5*time.Second, h, "GET", "/?json&q=match"))
		assert.ElementsMatch(t, []string{"match1.txt", "y/match3.txt"}, names)
		assert.True(t, incomplete)
	})

	t.Run("Complete", func(t *testing.T) {
		f.setListFault(nil)
		names, incomplete := decode(doWithin(t, 5*time.Second, h, "GET", "/?json&q=match"))
		assert.Len(t, names, 3)
		assert.False(t, incomplete)
	})

	t.Run("Hanging", func(t *testing.T) {
		VFS, f, _ := newFaultyVFS(t, map[string]string{"match1.txt": "1", "x/match2.txt": "2"})
		f.hangLists(t, "x")
		h := newFaultyUI(t, VFS, opt).Handler(&fallthroughHandler{})
		names, incomplete := decode(doWithin(t, 5*time.Second, h, "GET", "/?json&q=match"))
		assert.Equal(t, []string{"match1.txt"}, names)
		assert.True(t, incomplete)
	})
}

// TestDirCountBounded checks the directory counts of a listing are made
// in parallel and given up for a directory which hangs
func TestDirCountBounded(t *testing.T) {
	old := dirCountTimeout
	dirCountTimeout = 200 * time.Millisecond
	t.Cleanup(func() { dirCountTimeout = old })

	VFS, f, _ := newFaultyVFS(t, map[string]string{"a/1": "", "a/2": "", "b/1": "", "c/": ""})
	f.hangLists(t, "b")
	h := newFaultyUI(t, VFS, Options{DirCount: true}).Handler(&fallthroughHandler{})

	rec := doWithin(t, 5*time.Second, h, "GET", "/?json")
	require.Equal(t, http.StatusOK, rec.Code)
	var data IndexData
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &data))
	sizes := map[string]int64{}
	for _, p := range data.Paths {
		sizes[p.Name] = p.Size
	}
	assert.Equal(t, map[string]int64{"a": 2, "b": -1, "c": 0}, sizes)
}

// Test a directory whose listing hangs doesn't hold up the page of the
// directory which contains it
func TestParentPageWithHungChild(t *testing.T) {
	VFS, f, _ := newFaultyVFS(t, map[string]string{"d/a.txt": "a", "b.txt": "b"})
	f.hangLists(t, "d")
	h := newFaultyUI(t, VFS, Options{}).Handler(&fallthroughHandler{})

	// Start the listing of d, which carries on after the request gives up
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/d/?json", nil).WithContext(ctx))
	assert.Equal(t, http.StatusGatewayTimeout, rec.Code)

	rec = doWithin(t, 2*time.Second, h, "GET", "/?json")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"d"`)
	assert.Contains(t, rec.Body.String(), `"name":"b.txt"`)
}
