package webui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// resumeCleanupInterval is the minimum time between two cleanups of the
// stored positions
const resumeCleanupInterval = 24 * time.Hour

// mediaExts are the lowercased extensions of the files whose playback
// position is remembered
var mediaExts = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".mkv":  true,
	".mp3":  true,
	".ogg":  true,
	".wav":  true,
	".m4a":  true,
}

// Bodies of the replies of the resume endpoints
const (
	resumeDisabledBody   = `{"error":"Playback resume disabled"}`
	resumeNotFoundBody   = `{"error":"No resume position found"}`
	resumeLoadFailBody   = `{"error":"Failed to load position"}`
	resumeSaveFailBody   = `{"error":"Failed to save position"}`
	resumeClearFailBody  = `{"error":"Failed to clear position"}`
	resumeInvalidPosText = "Invalid position value"
)

// isMedia reports whether the file name has the extension of a media file
func isMedia(name string) bool {
	return mediaExts[strings.ToLower(path.Ext(name))]
}

// resumeRecord is the playback position of a file as it is stored and
// returned by ?resume-get
type resumeRecord struct {
	Position  float64  `json:"position"`  // seconds
	Duration  *float64 `json:"duration"`  // seconds, nil if unknown
	Timestamp int64    `json:"timestamp"` // Unix seconds of the last save
	FilePath  string   `json:"file_path"` // "/" + remote
}

// resumeStore keeps playback positions as files in a local directory
//
// There is one file per remote and path so the positions are shared by
// all the users of a remote. It is safe for concurrent use.
type resumeStore struct {
	dir       string
	retention time.Duration // positions older than this are cleaned up, 0 keeps them for ever

	mu          sync.Mutex // protects lastCleanup
	lastCleanup time.Time
}

// newResumeStore returns a store in dir, which is created if it doesn't
// exist
func newResumeStore(dir string, retention time.Duration) (*resumeStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &resumeStore{dir: dir, retention: retention}, nil
}

// resumeKey returns the name of the file holding the position of remote
// in the Fs described by configString
func resumeKey(configString, remote string) string {
	sum := sha256.Sum256([]byte(configString + "\x00" + remote))
	return hex.EncodeToString(sum[:]) + ".json"
}

// load returns the record stored under key. The error satisfies
// os.ErrNotExist if there is none.
func (s *resumeStore) load(key string) (*resumeRecord, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, key))
	if err != nil {
		return nil, err
	}
	var rec resumeRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("corrupt resume record %q: %w", key, err)
	}
	return &rec, nil
}

// save stores rec under key, replacing the record there.
//
// The record is written to a temporary file which is renamed over the old
// one so that a concurrent load never sees a partial record.
func (s *resumeStore) save(key string, rec *resumeRecord) (err error) {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	// CreateTemp makes the file readable by its owner only
	tmp, err := os.CreateTemp(s.dir, key+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	_, err = tmp.Write(data)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(s.dir, key))
}

// clear removes the record stored under key. It isn't an error if there
// is none.
func (s *resumeStore) clear(key string) error {
	err := os.Remove(filepath.Join(s.dir, key))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// cleanupDue reports whether a cleanup is due at now and if so records
// that one is starting
func (s *resumeStore) cleanupDue(now time.Time) bool {
	if s.retention <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastCleanup.IsZero() && now.Sub(s.lastCleanup) < resumeCleanupInterval {
		return false
	}
	s.lastCleanup = now
	return true
}

// cleanup removes the records last saved before now less the retention
// and returns how many it removed.
//
// The age of a record which can't be parsed is that of its file.
func (s *resumeStore) cleanup(now time.Time) (removed int) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		fs.Errorf(nil, "webui: playback resume cleanup failed: %v", err)
		return 0
	}
	cutoff := now.Add(-s.retention)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var saved time.Time
		if rec, err := s.load(entry.Name()); err == nil {
			saved = time.Unix(rec.Timestamp, 0)
		} else if info, err := entry.Info(); err == nil {
			saved = info.ModTime()
		} else {
			continue
		}
		if !saved.Before(cutoff) {
			continue
		}
		if err := s.clear(entry.Name()); err != nil {
			fs.Errorf(nil, "webui: failed to remove old playback position: %v", err)
			continue
		}
		removed++
	}
	return removed
}

// saved is called after a position is saved and starts a cleanup in the
// background if one is due
func (s *resumeStore) saved() {
	if s.cleanupDue(time.Now()) {
		go s.cleanup(time.Now())
	}
}

// resumeKeyFor returns the store key of remote in the VFS
func resumeKeyFor(VFS *vfs.VFS, remote string) string {
	return resumeKey(fs.ConfigString(VFS.Fs()), remote)
}

// resumePosition returns the saved position of the media file remote as
// shown in a listing, or nil if there is none
func (u *UI) resumePosition(VFS *vfs.VFS, remote string) *int64 {
	if u.resume == nil || !isMedia(remote) {
		return nil
	}
	rec, err := u.resume.load(resumeKeyFor(VFS, remote))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fs.Errorf(remote, "webui: failed to load playback position: %v", err)
		}
		return nil
	}
	pos := int64(rec.Position)
	return &pos
}

// resumeReply writes a JSON reply with the body, which is empty for a
// HEAD request
func resumeReply(w http.ResponseWriter, r *http.Request, status int, body string) {
	header := w.Header()
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Type", "application/json")
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(body))
	}
}

// finite reports whether f is neither infinite nor NaN
func finite(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

// serveResume handles the GET and HEAD requests of the playback resume
// API: ?resume-get, ?resume-save=<position>[&duration=<d>] and
// ?resume-clear. It returns false if the request isn't one of them and must
// be handled by the next handler.
func (u *UI) serveResume(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	query := r.URL.Query()
	switch {
	case query.Has("resume-get"), query.Has("resume-save"), query.Has("resume-clear"):
	default:
		return false
	}
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		return false
	}
	if u.resume == nil {
		resumeReply(w, r, http.StatusNotFound, resumeDisabledBody)
		return true
	}
	remote := remoteFromPath(r.URL.Path)
	key := resumeKeyFor(VFS, remote)
	switch {
	case query.Has("resume-get"):
		u.resumeGet(w, r, remote, key)
	case query.Has("resume-save"):
		u.resumeSave(w, r, VFS, remote, key)
	default:
		if err := u.resume.clear(key); err != nil {
			fs.Errorf(remote, "webui: failed to clear playback position: %v", err)
			resumeReply(w, r, http.StatusInternalServerError, resumeClearFailBody)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	}
	return true
}

// resumeGet replies with the saved position of remote
func (u *UI) resumeGet(w http.ResponseWriter, r *http.Request, remote, key string) {
	rec, err := u.resume.load(key)
	switch {
	case errors.Is(err, os.ErrNotExist):
		resumeReply(w, r, http.StatusNotFound, resumeNotFoundBody)
	case err != nil:
		fs.Errorf(remote, "webui: failed to load playback position: %v", err)
		resumeReply(w, r, http.StatusInternalServerError, resumeLoadFailBody)
	default:
		body, err := json.Marshal(rec)
		if err != nil {
			fs.Errorf(remote, "webui: failed to encode playback position: %v", err)
			resumeReply(w, r, http.StatusInternalServerError, resumeLoadFailBody)
			return
		}
		resumeReply(w, r, http.StatusOK, string(body))
	}
}

// resumeSave saves the position in the query for remote, which must be an
// existing media file
func (u *UI) resumeSave(w http.ResponseWriter, r *http.Request, VFS *vfs.VFS, remote, key string) {
	query := r.URL.Query()
	pos, err := strconv.ParseFloat(query.Get("resume-save"), 64)
	if err != nil || !finite(pos) || pos < 0 {
		httpError(w, http.StatusBadRequest, resumeInvalidPosText)
		return
	}
	var duration *float64
	if d, err := strconv.ParseFloat(query.Get("duration"), 64); err == nil && finite(d) {
		duration = &d
	}
	// Only existing media files may have a position so that requests can't
	// fill the store
	node, err := VFS.Stat(remote)
	if err != nil || node.IsDir() || !isMedia(remote) {
		httpError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}
	rec := resumeRecord{
		Position:  pos,
		Duration:  duration,
		Timestamp: time.Now().Unix(),
		FilePath:  "/" + remote,
	}
	if err := u.resume.save(key, &rec); err != nil {
		fs.Errorf(remote, "webui: failed to save playback position: %v", err)
		resumeReply(w, r, http.StatusInternalServerError, resumeSaveFailBody)
		return
	}
	u.resume.saved()
	w.WriteHeader(http.StatusNoContent)
}
