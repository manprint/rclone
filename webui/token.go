package webui

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// tokenSigLen is the number of bytes of the HMAC kept in a token
const tokenSigLen = 16

// keys are the secrets which sign the links which need no credentials
type keys struct {
	token   []byte // signs download tokens
	publink []byte // signs public link tokens
}

// deriveKeys returns the signing keys for the secret
//
// The keys depend only on the secret, so links survive a restart, if it
// isn't empty. If it is empty they come from a random secret which lives
// as long as the process.
func deriveKeys(secret string) (keys, error) {
	var master [sha256.Size]byte
	if secret != "" {
		master = sha256.Sum256([]byte(secret))
	} else if _, err := rand.Read(master[:]); err != nil {
		return keys{}, fmt.Errorf("failed to make a random secret: %w", err)
	}
	derive := func(label string) []byte {
		mac := hmac.New(sha256.New, master[:])
		_, _ = mac.Write([]byte(label))
		return mac.Sum(nil)
	}
	return keys{
		token:   derive("webui-token"),
		publink: derive("webui-publink"),
	}, nil
}

// tokenSignature returns the hex signature of a download token for user
// of the remote which expires at expMs milliseconds since the Unix epoch
func (u *UI) tokenSignature(user, remote string, expMs int64) string {
	mac := hmac.New(sha256.New, u.keys.token)
	// NUL can't be in a remote's path segment, so it separates the fields
	// without ambiguity (see checkToken for the user).
	_, _ = mac.Write([]byte(user + "\x00" + remote + "\x00" + strconv.FormatInt(expMs, 10)))
	return hex.EncodeToString(mac.Sum(nil)[:tokenSigLen])
}

// makeToken returns a download token for user to read the remote, which
// is valid until the token expiry after now
//
// The token has the form expMs.base64(user).signature and is valid only
// for the exact remote, not for the entries below it.
func (u *UI) makeToken(user, remote string, now time.Time) string {
	expMs := now.Add(time.Duration(u.opt.TokenExpiry)).UnixMilli()
	return strconv.FormatInt(expMs, 10) +
		"." + base64.RawURLEncoding.EncodeToString([]byte(user)) +
		"." + u.tokenSignature(user, remote, expMs)
}

// checkToken returns the user the download token was made for and true
// if tok is a token which is valid at now for the remote
//
// The token is parsed strictly, so that it has only one spelling, and its
// signature is compared in constant time.
func (u *UI) checkToken(tok, remote string, now time.Time) (user string, ok bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", false
	}
	expMs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || strconv.FormatInt(expMs, 10) != parts[0] {
		return "", false
	}
	if now.UnixMilli() > expMs {
		return "", false
	}
	rawUser, err := base64.RawURLEncoding.Strict().DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	user = string(rawUser)
	// A user with a NUL could pass for a different user and remote.
	if strings.Contains(user, "\x00") {
		return "", false
	}
	want := u.tokenSignature(user, remote, expMs)
	if !hmac.Equal([]byte(parts[2]), []byte(want)) {
		return "", false
	}
	return user, true
}

// tokensUsable reports whether the server can check download tokens
//
// They stand in for credentials so need an auth which isn't the proxy,
// as only without it does the VFS not depend on the user.
func (u *UI) tokensUsable() bool {
	return u.cfg.UsingAuth() && !u.cfg.IsProxy
}

// serveTokengen handles the GET requests for a token for the node in the
// URL path. It returns false if the request isn't one for a token.
//
// The reply is empty if the server doesn't check tokens, as the client
// has then no use for one and the browser sends its credentials anyway.
func (u *UI) serveTokengen(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet || !r.URL.Query().Has("tokengen") {
		return false
	}
	remote := remoteFromPath(r.URL.Path)
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		return false
	}
	if _, err := VFS.Stat(remote); err != nil {
		return false
	}
	body := ""
	if u.tokensUsable() {
		body = u.makeToken(u.user(r), remote, time.Now())
	}
	header := w.Header()
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
	return true
}

// tokenRequest returns the remote and the user of a request carrying a
// valid download token, and true
//
// It is true only for a GET or HEAD request with no credentials of its
// own, for a path below the base URL which the server accepts.
func (u *UI) tokenRequest(r *http.Request) (remote, user string, ok bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", "", false
	}
	query := r.URL.Query()
	if !query.Has("token") || !u.tokensUsable() {
		return "", "", false
	}
	// Credentials the auth middleware must decide on, not the token
	if len(r.Header.Values("Authorization")) != 0 {
		return "", "", false
	}
	rel := u.relPath(r)
	if rel == "" || u.cfg.CheckPath(rel) != nil {
		return "", "", false
	}
	remote = remoteFromPath(rel)
	user, ok = u.checkToken(query.Get("token"), remote, time.Now())
	if !ok {
		return "", "", false
	}
	return remote, user, true
}

// tokenDownload returns the handler which serves a request carrying a
// valid download token, or nil if the request must be left to the auth
// middleware
//
// The token is for a file, or for a directory which is zipped. A request
// for any other use of it isn't served.
func (u *UI) tokenDownload(r *http.Request) http.Handler {
	remote, user, ok := u.tokenRequest(r)
	if !ok {
		return nil
	}
	VFS, err := u.cfg.GetVFS(r.Context())
	if err != nil {
		return nil
	}
	node, statErr := VFS.Stat(remote)
	var serve func(w http.ResponseWriter, r *http.Request)
	switch {
	case statErr != nil:
		serve = func(w http.ResponseWriter, r *http.Request) { vfsError(w, remote, statErr) }
	case node.IsDir():
		query := r.URL.Query()
		if !query.Has("zip") {
			return nil
		}
		serve = func(w http.ResponseWriter, r *http.Request) {
			fs.Infof(remote, "webui: token download for user %q", user)
			u.serveZip(w, r, VFS, remote, query)
		}
	case strings.HasSuffix(r.URL.Path, "/"):
		return nil
	default:
		serve = func(w http.ResponseWriter, r *http.Request) {
			fs.Infof(remote, "webui: token download for user %q", user)
			serveFileRaw(w, r, VFS, remote)
		}
	}
	return u.publicWrap(u.hold(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The token is in the URL, so keep it from leaking in a Referer
		w.Header().Set("Referrer-Policy", "no-referrer")
		serve(w, r)
	})))
}
