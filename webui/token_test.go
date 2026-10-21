package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTokenUI returns a UI for the tests of tokens
func newTokenUI(t *testing.T, opt Options, mutate func(*Config)) *UI {
	t.Helper()
	cfg := Config{
		BaseURL:   func() string { return "/" },
		CheckPath: func(string) error { return nil },
		UsingAuth: func() bool { return true },
	}
	if mutate != nil {
		mutate(&cfg)
	}
	u, err := New(cfg, opt)
	require.NoError(t, err)
	return u
}

// TestTokenRoundTrip checks a token is accepted for its remote and gives its user
func TestTokenRoundTrip(t *testing.T) {
	u := newTokenUI(t, Options{TokenExpiry: fs.Duration(time.Hour)}, nil)
	now := time.UnixMilli(1_700_000_000_000)

	for _, user := range []string{"alice", "", "bob@example.com", "ünï.cödé", "a.b.c"} {
		tok := u.makeToken(user, "dir/f.txt", now)
		got, ok := u.checkToken(tok, "dir/f.txt", now)
		assert.True(t, ok, user)
		assert.Equal(t, user, got)
	}

	tok := u.makeToken("alice", "f.txt", now)
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 3)
	assert.Equal(t, "1700003600000", parts[0])
	assert.Len(t, parts[2], 32)

	// Valid up to and including the expiry
	_, ok := u.checkToken(tok, "f.txt", now.Add(time.Hour))
	assert.True(t, ok)
	_, ok = u.checkToken(tok, "f.txt", now.Add(time.Hour+time.Millisecond))
	assert.False(t, ok)
}

// TestTokenRejects checks the tokens which must not be accepted
func TestTokenRejects(t *testing.T) {
	u := newTokenUI(t, Options{TokenExpiry: fs.Duration(time.Hour)}, nil)
	now := time.UnixMilli(1_700_000_000_000)
	tok := u.makeToken("alice", "d/f.txt", now)
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 3)
	exp, user, sig := parts[0], parts[1], parts[2]

	flip := func(s string) string {
		b := []byte(s)
		if b[len(b)-1] == 'a' {
			b[len(b)-1] = 'b'
		} else {
			b[len(b)-1] = 'a'
		}
		return string(b)
	}
	// The same user spelt with other bits in the unused end of the last
	// base64 character, which a lenient decoder doesn't tell apart
	require.Equal(t, "YWxpY2U", user)
	altUser := "YWxpY2V"

	for _, test := range []struct {
		name   string
		tok    string
		remote string
		now    time.Time
	}{
		{"Valid", tok, "d/f.txt", now},
		{"Expired", tok, "d/f.txt", now.Add(2 * time.Hour)},
		{"SignatureAltered", exp + "." + user + "." + flip(sig), "d/f.txt", now},
		{"SignatureUppercase", exp + "." + user + "." + strings.ToUpper(sig), "d/f.txt", now},
		{"SignatureShort", exp + "." + user + "." + sig[:31], "d/f.txt", now},
		{"SignatureLong", exp + "." + user + "." + sig + "0", "d/f.txt", now},
		{"SignatureEmpty", exp + "." + user + ".", "d/f.txt", now},
		{"UserAltered", exp + "." + "Ym9i" + "." + sig, "d/f.txt", now},
		{"UserAlternativeSpelling", exp + "." + altUser + "." + sig, "d/f.txt", now},
		{"UserBase64Invalid", exp + ".!!!." + sig, "d/f.txt", now},
		{"UserBase64Padded", exp + "." + user + "=." + sig, "d/f.txt", now},
		{"ExpiryAltered", "1700003600001." + user + "." + sig, "d/f.txt", now},
		{"ExpiryNotNumber", "soon." + user + "." + sig, "d/f.txt", now},
		{"ExpiryPlusSign", "+" + exp + "." + user + "." + sig, "d/f.txt", now},
		{"ExpiryLeadingZero", "0" + exp + "." + user + "." + sig, "d/f.txt", now},
		{"ExpiryOverflow", "99999999999999999999." + user + "." + sig, "d/f.txt", now},
		{"OtherRemote", tok, "d/g.txt", now},
		{"ChildOfRemote", tok, "d/f.txt/x", now},
		{"ParentOfRemote", tok, "d", now},
		{"TwoParts", exp + "." + sig, "d/f.txt", now},
		{"FourParts", tok + ".x", "d/f.txt", now},
		{"Empty", "", "d/f.txt", now},
		{"Dots", "..", "d/f.txt", now},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := u.checkToken(test.tok, test.remote, test.now)
			if test.name == "Valid" {
				assert.True(t, ok)
				assert.Equal(t, "alice", got)
				return
			}
			assert.False(t, ok)
			assert.Empty(t, got)
		})
	}

	t.Run("UserWithNUL", func(t *testing.T) {
		// A user which could be confused with another user and remote
		tok := u.makeToken("a\x00b", "c", now)
		_, ok := u.checkToken(tok, "c", now)
		assert.False(t, ok)
	})
	t.Run("OtherServer", func(t *testing.T) {
		other := newTokenUI(t, Options{TokenExpiry: fs.Duration(time.Hour)}, nil)
		_, ok := other.checkToken(tok, "d/f.txt", now)
		assert.False(t, ok)
	})
	t.Run("PublicLinkKeyIsNotTokenKey", func(t *testing.T) {
		assert.NotEqual(t, u.keys.token, u.keys.publink)
	})
}

// TestKeysFromSecret checks the keys depend on the secret, and are random without one
func TestKeysFromSecret(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	mk := func(secret string) *UI {
		return newTokenUI(t, Options{Secret: secret, TokenExpiry: fs.Duration(time.Hour)}, nil)
	}
	tokenOf := func(u *UI) string { return u.makeToken("alice", "f.txt", now) }

	assert.Equal(t, tokenOf(mk("one")), tokenOf(mk("one")))
	assert.NotEqual(t, tokenOf(mk("one")), tokenOf(mk("two")))
	assert.Equal(t, mk("one").keys, mk("one").keys)
	assert.NotEqual(t, mk("one").keys.token, mk("one").keys.publink)

	// A secret which is empty gives each server its own random keys
	assert.NotEqual(t, tokenOf(mk("")), tokenOf(mk("")))
	assert.NotEqual(t, mk("").keys.publink, mk("").keys.publink)
	assert.NotEqual(t, tokenOf(mk("")), tokenOf(mk("one")))

	// A token from one server isn't valid on another with a different secret
	_, ok := mk("two").checkToken(tokenOf(mk("one")), "f.txt", now)
	assert.False(t, ok)
	_, ok = mk("one").checkToken(tokenOf(mk("one")), "f.txt", now)
	assert.True(t, ok)
}

// TestPreAuthTokenIgnoredInProxy checks a valid token is not honoured when
// using an auth proxy, with no auth or next to other credentials, and is
// when none of them apply
func TestPreAuthTokenIgnoredInProxy(t *testing.T) {
	opt := Options{TokenExpiry: fs.Duration(time.Hour)}
	for _, test := range []struct {
		name      string
		proxy     bool
		usingAuth bool
		header    []string
		wantNext  bool
	}{
		{"Proxy", true, true, nil, true},
		{"NoAuth", false, false, nil, true},
		{"AuthorizationHeader", false, true, []string{"Authorization", "Basic YTpi"}, true},
		{"Honoured", false, true, nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			u := newTokenUI(t, opt, func(c *Config) {
				c.IsProxy = test.proxy
				c.UsingAuth = func() bool { return test.usingAuth }
			})
			tok := u.makeToken("alice", "f.txt", time.Now())
			next := &fallthroughHandler{}
			h := u.PreAuth(next)
			if test.wantNext {
				rec := do(h, "GET", "/f.txt?token="+tok, test.header...)
				assert.Equal(t, http.StatusTeapot, rec.Code)
				assert.Equal(t, []string{"/f.txt"}, next.paths)
				req := httptest.NewRequest("GET", "/f.txt?token="+tok, nil)
				for i := 0; i < len(test.header); i += 2 {
					req.Header.Set(test.header[i], test.header[i+1])
				}
				_, _, ok := u.tokenRequest(req)
				assert.False(t, ok)
				return
			}
			req := httptest.NewRequest("GET", "/f.txt?token="+tok, nil)
			remote, user, ok := u.tokenRequest(req)
			assert.True(t, ok)
			assert.Equal(t, "f.txt", remote)
			assert.Equal(t, "alice", user)
		})
	}
}
