package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout   = 500 * time.Millisecond
	testChunkGap  = 50 * time.Millisecond
	testChunks    = 30 // testChunks*testChunkGap is 3 timeouts
	testChunkSize = 64 * 1024
)

// testTimeoutServer starts a server with short read and write timeouts
func testTimeoutServer(t *testing.T, handler http.HandlerFunc) string {
	cfg := DefaultCfg()
	cfg.ListenAddr = []string{"127.0.0.1:0"}
	cfg.ServerReadTimeout = fs.Duration(testTimeout)
	cfg.ServerWriteTimeout = fs.Duration(testTimeout)
	s, err := NewServer(context.Background(), WithConfig(cfg))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Shutdown()) })
	s.Router().Handle("/", handler)
	s.Serve()
	return testGetServerURL(t, s)
}

func testTimeoutClients() map[string]*http.Client {
	var h2c http.Protocols
	h2c.SetUnencryptedHTTP2(true)
	return map[string]*http.Client{
		"HTTP1": {Transport: &http.Transport{}},
		"H2C":   {Transport: &http.Transport{Protocols: &h2c}},
	}
}

// slowBody sends chunks with a gap between them, stalling forever after stallAfter chunks if it is > 0
func slowBody(ctx context.Context, chunks, stallAfter int) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		chunk := make([]byte, testChunkSize)
		for i := range chunks {
			if stallAfter > 0 && i == stallAfter {
				<-ctx.Done()
				_ = pw.CloseWithError(ctx.Err())
				return
			}
			time.Sleep(testChunkGap)
			if _, err := pw.Write(chunk); err != nil {
				return
			}
		}
		_ = pw.Close()
	}()
	return pr
}

// A slow upload which keeps making progress takes longer than the server timeouts
func TestServerTimeoutSlowUpload(t *testing.T) {
	url := testTimeoutServer(t, func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(fs.SizeSuffix(n).String()))
	})
	for name, client := range testTimeoutClients() {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest("PUT", url, slowBody(t.Context(), testChunks, 0))
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			testExpectRespBody(t, resp, []byte(fs.SizeSuffix(testChunks*testChunkSize).String()))
		})
	}
}

// A slow download which keeps making progress takes longer than the server timeouts
func TestServerTimeoutSlowDownload(t *testing.T) {
	url := testTimeoutServer(t, func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, testChunkSize)
		for range testChunks {
			time.Sleep(testChunkGap)
			if _, err := w.Write(chunk); err != nil {
				return
			}
			_ = http.NewResponseController(w).Flush()
		}
	})
	for name, client := range testTimeoutClients() {
		t.Run(name, func(t *testing.T) {
			resp, err := client.Get(url)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			n, err := io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			assert.Equal(t, int64(testChunks*testChunkSize), n)
		})
	}
}

// An upload which stops making progress is still cut off by the read timeout
func TestServerTimeoutStalledUpload(t *testing.T) {
	readErr := make(chan error, 2)
	url := testTimeoutServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := io.Copy(io.Discard, r.Body)
		readErr <- err
	})
	for name, client := range testTimeoutClients() {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "PUT", url, slowBody(ctx, testChunks, 2))
			require.NoError(t, err)
			go func() {
				resp, err := client.Do(req)
				if err == nil {
					_ = resp.Body.Close()
				}
			}()
			select {
			case err := <-readErr:
				require.Error(t, err)
				var netErr net.Error
				if errors.As(err, &netErr) {
					assert.True(t, netErr.Timeout(), "expecting a timeout, got %v", err)
				}
			case <-time.After(10 * testTimeout):
				t.Fatal("stalled upload was not cut off by the read timeout")
			}
		})
	}
}
