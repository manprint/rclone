package webui

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSelection checks the validation of the paths of a selection zip
func TestParseSelection(t *testing.T) {
	many := make([]string, 201)
	for i := range many {
		many[i] = `"f` + strconv.Itoa(i) + `"`
	}
	for _, test := range []struct {
		name    string
		raw     string
		want    []string
		wantMsg string
	}{
		{"Missing", "", nil, "Missing paths parameter"},
		{"TooLarge", `["` + strings.Repeat("a", maxSelectionPayload) + `"]`, nil, "Selection payload too large"},
		{"NotJSON", `a.txt`, nil, "Invalid selection payload"},
		{"NotStrings", `[1,2]`, nil, "Invalid selection payload"},
		{"NotArray", `{"a":1}`, nil, "Invalid selection payload"},
		{"EmptyList", `[]`, nil, "Selection is empty"},
		{"TooMany", "[" + strings.Join(many, ",") + "]", nil, "Too many selected paths"},
		{"OnlySlashes", `["","/","//"]`, nil, "Selection is empty"},
		{"DotDot", `["../x"]`, nil, "Invalid selection path"},
		{"DotDotInside", `["a/../b"]`, nil, "Invalid selection path"},
		{"DotDotLast", `["a/.."]`, nil, "Invalid selection path"},
		{"Dot", `["./a"]`, nil, "Invalid selection path"},
		{"EmptySegment", `["a//b"]`, nil, "Invalid selection path"},
		{"Backslash", `["a\\b"]`, nil, "Invalid selection path"},
		{"InvalidAfterValid", `["a","..\\x"]`, nil, "Invalid selection path"},
		{"Plain", `["a.txt","d"]`, []string{"a.txt", "d"}, ""},
		{"TrimSlashes", `["/a.txt","d/","/e/f/"]`, []string{"a.txt", "d", "e/f"}, ""},
		{"DedupKeepsOrder", `["b","a","b/","/a","c"]`, []string{"b", "a", "c"}, ""},
		{"SkipsEmpty", `["","a","/"]`, []string{"a"}, ""},
		{"DotsInNames", `["..a","a..","...","a.b/.c"]`, []string{"..a", "a..", "...", "a.b/.c"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, status, msg := parseSelection(test.raw, 200)
			assert.Equal(t, test.wantMsg, msg)
			if test.wantMsg != "" {
				assert.Equal(t, http.StatusBadRequest, status)
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, http.StatusOK, status)
			assert.Equal(t, test.want, got)
		})
	}

	t.Run("MaxIsInclusive", func(t *testing.T) {
		got, _, msg := parseSelection(`["a","b"]`, 2)
		assert.Empty(t, msg)
		assert.Len(t, got, 2)
		_, _, msg = parseSelection(`["a","b","c"]`, 2)
		assert.Equal(t, "Too many selected paths", msg)
	})
}

// newTestVFS returns a VFS on a temporary directory holding the files
// named by the keys of content
func newTestVFS(t *testing.T, content map[string]string) (*vfs.VFS, string) {
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
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeOff
	VFS := vfs.New(context.Background(), f, &opt)
	t.Cleanup(VFS.Shutdown)
	return VFS, dir
}

// readZip returns the contents of a zip by entry name, with the entry
// order and the compression method of each entry
func readZip(t *testing.T, data []byte) (contents map[string]string, order []string, methods map[string]uint16) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	contents = map[string]string{}
	methods = map[string]uint16{}
	for _, file := range zr.File {
		rc, err := file.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		contents[file.Name] = string(body)
		methods[file.Name] = file.Method
		order = append(order, file.Name)
	}
	return contents, order, methods
}

// TestWriteSelectionZip checks the entries of a selection zip
func TestWriteSelectionZip(t *testing.T) {
	VFS, dir := newTestVFS(t, map[string]string{
		"a.txt":     "aaa",
		"d/b.txt":   "bbbb",
		"d/e/c.txt": "c",
		"d/empty/":  "",
		"other.txt": "no",
		"top/x.txt": "x",
	})
	modTime := time.Date(2020, 1, 2, 3, 4, 6, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "a.txt"), modTime, modTime))

	t.Run("FilesAndDirectories", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeSelectionZip(context.Background(), &buf, VFS, "", []string{"a.txt", "d"}))
		contents, order, methods := readZip(t, buf.Bytes())
		assert.Equal(t, map[string]string{
			"a.txt":     "aaa",
			"d/":        "",
			"d/b.txt":   "bbbb",
			"d/e/":      "",
			"d/e/c.txt": "c",
			"d/empty/":  "",
		}, contents)
		assert.Equal(t, "a.txt", order[0])
		assert.Equal(t, "d/", order[1])
		assert.Equal(t, zip.Deflate, methods["a.txt"])
		assert.Equal(t, zip.Store, methods["d/"])
		assert.NotContains(t, contents, "other.txt")

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)
		assert.True(t, zr.File[0].Modified.Equal(modTime), "modtime %v", zr.File[0].Modified)
	})

	t.Run("RelativeToDirectory", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, writeSelectionZip(context.Background(), &buf, VFS, "d", []string{"e", "b.txt"}))
		contents, _, _ := readZip(t, buf.Bytes())
		assert.Equal(t, map[string]string{
			"e/":      "",
			"e/c.txt": "c",
			"b.txt":   "bbbb",
		}, contents)
	})

	t.Run("MissingEntry", func(t *testing.T) {
		var buf bytes.Buffer
		err := writeSelectionZip(context.Background(), &buf, VFS, "", []string{"a.txt", "nope"})
		require.Error(t, err)
		_, zerr := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		assert.Error(t, zerr, "a failed zip has no central directory")
	})

	t.Run("Cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var buf bytes.Buffer
		assert.ErrorIs(t, writeSelectionZip(ctx, &buf, VFS, "", []string{"a.txt"}), context.Canceled)
	})
}
