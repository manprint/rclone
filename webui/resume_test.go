package webui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumeStoreRoundTrip checks a saved record is loaded back and the
// store is private to its owner
func TestResumeStoreRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "resume")
	s, err := newResumeStore(dir, time.Hour)
	require.NoError(t, err)

	key := resumeKey("local:/data", "dir/v.mp4")
	_, err = s.load(key)
	assert.ErrorIs(t, err, os.ErrNotExist)

	duration := 100.0
	rec := &resumeRecord{Position: 12.5, Duration: &duration, Timestamp: 1700000000, FilePath: "/dir/v.mp4"}
	require.NoError(t, s.save(key, rec))
	got, err := s.load(key)
	require.NoError(t, err)
	assert.Equal(t, rec, got)

	// A record without a duration keeps it null
	rec2 := &resumeRecord{Position: 1, Timestamp: 1700000001, FilePath: "/x.mp3"}
	require.NoError(t, s.save(key, rec2))
	raw, err := os.ReadFile(filepath.Join(dir, key))
	require.NoError(t, err)
	assert.JSONEq(t, `{"position":1,"duration":null,"timestamp":1700000001,"file_path":"/x.mp3"}`, string(raw))

	require.NoError(t, s.clear(key))
	_, err = s.load(key)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.NoError(t, s.clear(key), "clearing a missing record is not an error")

	// No temporary file is left behind
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)

	if runtime.GOOS != "windows" {
		require.NoError(t, s.save(key, rec))
		info, err := os.Stat(filepath.Join(dir, key))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
		info, err = os.Stat(dir)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0700), info.Mode().Perm())
	}

	t.Run("Corrupt", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, key), []byte("{"), 0600))
		_, err := s.load(key)
		require.Error(t, err)
		assert.NotErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("Concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range 50 {
					assert.NoError(t, s.save(key, &resumeRecord{Position: float64(i*100 + j), Timestamp: 1}))
					if _, err := s.load(key); err != nil {
						assert.NoError(t, err)
					}
				}
			}()
		}
		wg.Wait()
		_, err := s.load(key)
		assert.NoError(t, err)
	})
}

// TestResumeKeyDistinct checks the key depends on the Fs and the path
func TestResumeKeyDistinct(t *testing.T) {
	a := resumeKey("local:/a", "v.mp4")
	assert.Equal(t, a, resumeKey("local:/a", "v.mp4"))
	assert.NotEqual(t, a, resumeKey("local:/b", "v.mp4"))
	assert.NotEqual(t, a, resumeKey("local:/a", "w.mp4"))
	assert.NotEqual(t, a, resumeKey("local:/a", "d/v.mp4"))
	assert.Regexp(t, `^[0-9a-f]{64}\.json$`, a)
}

// TestResumeCleanup checks old records are removed and recent ones kept
func TestResumeCleanup(t *testing.T) {
	dir := t.TempDir()
	s, err := newResumeStore(dir, 24*time.Hour)
	require.NoError(t, err)
	now := time.Now()

	require.NoError(t, s.save("old.json", &resumeRecord{Timestamp: now.Add(-48 * time.Hour).Unix()}))
	require.NoError(t, s.save("new.json", &resumeRecord{Timestamp: now.Add(-time.Hour).Unix()}))
	// The age of a record which can't be parsed is that of its file
	corruptOld := filepath.Join(dir, "corrupt-old.json")
	corruptNew := filepath.Join(dir, "corrupt-new.json")
	for _, p := range []string{corruptOld, corruptNew} {
		require.NoError(t, os.WriteFile(p, []byte("not json"), 0600))
	}
	require.NoError(t, os.Chtimes(corruptOld, now, now.Add(-48*time.Hour)))
	// Other files are left alone
	other := filepath.Join(dir, "other.txt")
	require.NoError(t, os.WriteFile(other, []byte("x"), 0600))
	require.NoError(t, os.Chtimes(other, now, now.Add(-48*time.Hour)))

	assert.Equal(t, 2, s.cleanup(now))
	names := []string{}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"new.json", "corrupt-new.json", "other.txt"}, names)

	t.Run("Due", func(t *testing.T) {
		s := &resumeStore{dir: dir, retention: time.Hour}
		assert.True(t, s.cleanupDue(now), "first save")
		assert.False(t, s.cleanupDue(now.Add(time.Hour)))
		assert.False(t, s.cleanupDue(now.Add(23*time.Hour)))
		assert.True(t, s.cleanupDue(now.Add(25*time.Hour)))
		assert.False(t, s.cleanupDue(now.Add(26*time.Hour)))
	})

	t.Run("NoRetention", func(t *testing.T) {
		s := &resumeStore{dir: dir}
		assert.False(t, s.cleanupDue(now))
	})
}

// TestIsMedia checks the media extensions
func TestIsMedia(t *testing.T) {
	for name, want := range map[string]bool{
		"v.mp4":                           true,
		"V.MP4":                           true,
		"dir/a.b.mkv":                     true,
		"a.mov":                           true,
		"a.avi":                           true,
		"a.wmv":                           true,
		"a.flv":                           true,
		"a.webm":                          true,
		"a.mp3":                           true,
		"a.ogg":                           true,
		"a.wav":                           true,
		"a.m4a":                           true,
		"a.txt":                           false,
		"a.mp4.txt":                       false,
		"mp4":                             false,
		".mp4x":                           false,
		"a.flac":                          false,
		"":                                false,
		strings.Repeat("x", 300) + ".mkv": true,
	} {
		assert.Equal(t, want, isMedia(name), name)
	}
}
