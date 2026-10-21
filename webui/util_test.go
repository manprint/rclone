package webui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDestRemote checks the remote taken from the Destination header
func TestDestRemote(t *testing.T) {
	for _, test := range []struct {
		name    string
		baseURL string
		host    string // Host of the request
		dest    string
		want    string
		wantOK  bool
	}{
		{"Plain", "", "h", "http://h/a/b", "a/b", true},
		{"PathOnly", "/", "h", "/a/b", "a/b", true},
		{"Root", "", "h", "http://h/", "", true},
		{"DirSlash", "", "h", "/a/b/", "a/b", true},
		{"OtherHost", "", "h", "http://other/a/b", "", false},
		{"SameHostWithPort", "", "h:8000", "http://h:8000/a", "a", true},
		{"PortDiffers", "", "h:8000", "http://h:9000/a", "", false},
		{"BaseURL", "/pfx", "h", "http://h/pfx/a/b", "a/b", true},
		{"BaseURLUnnormalized", "pfx/", "h", "/pfx/a/b", "a/b", true},
		{"BaseURLRoot", "/pfx", "h", "/pfx/", "", true},
		{"OutsideBaseURL", "/pfx", "h", "/other/a", "", false},
		{"BaseURLNoSlash", "/pfx", "h", "/pfxother/a", "", false},
		{"BaseURLItself", "/pfx", "h", "/pfx", "", false},
		{"DotDot", "", "h", "/a/../b", "", false},
		{"EncodedDotDot", "", "h", "/%2e%2e/b", "", false},
		{"DotDotAfterBaseURL", "/pfx", "h", "/pfx/../b", "", false},
		{"Unparsable", "", "h", "http://h/%zz", "", false},
		{"Empty", "", "h", "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			u, err := New(Config{
				BaseURL: func() string { return test.baseURL },
				CheckPath: func(name string) error {
					for _, elem := range strings.Split(name, "/") {
						if elem == "." || elem == ".." {
							return errors.New("bad path")
						}
					}
					return nil
				},
			}, Options{})
			require.NoError(t, err)
			req := httptest.NewRequest("MOVE", "/src", nil)
			req.Host = test.host
			req.Header.Set("Destination", test.dest)
			got, ok := u.destRemote(req)
			assert.Equal(t, test.wantOK, ok)
			assert.Equal(t, test.want, got)
		})
	}
}

// TestErrStatus checks the mapping of VFS errors to HTTP statuses
func TestErrStatus(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, errStatus(vfs.ENOENT))
	assert.Equal(t, http.StatusNotFound, errStatus(fmt.Errorf("wrapped: %w", os.ErrNotExist)))
	assert.Equal(t, http.StatusForbidden, errStatus(vfs.EPERM))
	assert.Equal(t, http.StatusForbidden, errStatus(vfs.EROFS))
	assert.Equal(t, http.StatusInternalServerError, errStatus(errors.New("boom")))
}
