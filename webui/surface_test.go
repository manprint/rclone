package webui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedQueries are the query parameters used by assets/index.js, each
// with who answers it.
var expectedQueries = map[string]string{
	"zip":          "webui zip.go and listing.go; ?download=zip is upstream's",
	"paths":        "webui zip.go, the selection of ?zip=multi",
	"q":            "webui listing.go, the search",
	"edit":         "webui edit.go",
	"view":         "webui edit.go and public.go",
	"playlist":     "client only: read by the page from location.search, ignored by the server",
	"tokengen":     "webui token.go",
	"token":        "webui token.go",
	"disk-stats":   "webui listing.go",
	"resume-get":   "webui resume.go",
	"resume-save":  "webui resume.go",
	"duration":     "webui resume.go, sent with resume-save",
	"resume-clear": "webui resume.go",
	"json":         "webui listing.go and public.go",
	"login":        "webui write.go, the variant of CHECKAUTH",
	"sort":         "webui listing.go and public.go, sent in the column header links",
	"order":        "webui listing.go and public.go, sent in the column header links",
	"t":            "cache buster of the video URL, ignored by the server",
}

// expectedMethods are the HTTP methods used by assets/index.js, each with
// who answers it.
var expectedMethods = map[string]string{
	"GET":       "webui listing.go and edit.go, upstream for the files",
	"HEAD":      "webui listing.go and edit.go, upstream for the files",
	"PUT":       "upstream; webui write.go checks the Origin",
	"PATCH":     "webui write.go, appends to a file",
	"DELETE":    "upstream; webui write.go checks the Origin",
	"MKCOL":     "upstream; webui write.go checks the Origin",
	"MOVE":      "upstream; webui write.go checks the Origin",
	"CHECKAUTH": "webui write.go",
	"LOGOUT":    "webui write.go",
}

var (
	// queryRe matches a query parameter inside a URL built in a string
	// literal. The trailing ';' of group 2 tells an HTML entity from a
	// parameter.
	queryRe = regexp.MustCompile(`[?&]([a-z][a-z-]*)(?:=|\b)(;?)`)

	// searchParamsSetRe matches a parameter set on a URL object
	searchParamsSetRe = regexp.MustCompile(`searchParams\.set\(["']([a-z][a-z-]*)["']`)

	// checkAuthRe matches the variant of CHECKAUTH, which the page sends
	// as the query of its URL
	checkAuthRe = regexp.MustCompile(`checkAuth\(["']([a-z][a-z-]*)["']\)`)

	// paramsObjectRe matches the object passed to URLSearchParams
	paramsObjectRe = regexp.MustCompile(`URLSearchParams\(\{([^{}]*)\}\)`)

	// methodRe matches the HTTP method of a fetch and of an XMLHttpRequest
	methodRe = regexp.MustCompile(`method:\s*["']([A-Z]+)["']|\.open\(["']([A-Z]+)["']`)
)

// commentLineRe matches a line holding only a comment
var commentLineRe = regexp.MustCompile(`^\s*(//|/\*|\*)`)

// extractSurface returns the sorted names of the query parameters and
// HTTP methods used by the script js
func extractSurface(js string) (queries, methods []string) {
	var code []string
	for _, line := range strings.Split(js, "\n") {
		if !commentLineRe.MatchString(line) {
			code = append(code, line)
		}
	}
	src := strings.Join(code, "\n")

	q := map[string]struct{}{}
	for _, m := range queryRe.FindAllStringSubmatch(src, -1) {
		if m[2] == "" {
			q[m[1]] = struct{}{}
		}
	}
	for _, re := range []*regexp.Regexp{searchParamsSetRe, checkAuthRe} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			q[m[1]] = struct{}{}
		}
	}
	for _, m := range paramsObjectRe.FindAllStringSubmatch(src, -1) {
		for _, entry := range strings.Split(m[1], ",") {
			key, _, _ := strings.Cut(strings.TrimSpace(entry), ":")
			if key = strings.TrimSpace(key); key != "" && !strings.HasPrefix(key, "...") {
				q[key] = struct{}{}
			}
		}
	}

	me := map[string]struct{}{}
	for _, m := range methodRe.FindAllStringSubmatch(src, -1) {
		me[m[1]+m[2]] = struct{}{}
	}
	return sortedKeys(q), sortedKeys(me)
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func expectedNames(expected map[string]string) []string {
	names := make([]string, 0, len(expected))
	for k := range expected {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// diffNames returns the names in got which aren't in want and those in want
// which aren't in got
func diffNames(got, want []string) (added, vanished []string) {
	for _, n := range got {
		if !slices.Contains(want, n) {
			added = append(added, n)
		}
	}
	for _, n := range want {
		if !slices.Contains(got, n) {
			vanished = append(vanished, n)
		}
	}
	return added, vanished
}

// surfaceDiffMessage returns the failure message for a change of the
// surface of kind, or "" if there is none
func surfaceDiffMessage(kind string, got, want []string) string {
	added, vanished := diffNames(got, want)
	if len(added) == 0 && len(vanished) == 0 {
		return ""
	}
	return fmt.Sprintf("%s used by assets/index.js changed\n  new (not handled by the server?): %v\n  vanished (no longer used?): %v\n"+
		"see fork-doc/sviluppo.md, section Sincronizzazione, for how to handle an update of the assets",
		kind, added, vanished)
}

// TestAPISurfaceLock checks that the query parameters and HTTP methods
// used by the embedded index.js are the ones the server handles.
func TestAPISurfaceLock(t *testing.T) {
	t.Run("Extractor", testSurfaceExtractor)
	t.Run("Assets", func(t *testing.T) {
		data, err := assetFS.ReadFile("assets/index.js")
		require.NoError(t, err)
		queries, methods := extractSurface(string(data))
		t.Logf("queries: %v", queries)
		t.Logf("methods: %v", methods)

		if msg := surfaceDiffMessage("query parameters", queries, expectedNames(expectedQueries)); msg != "" {
			t.Error(msg)
		}
		if msg := surfaceDiffMessage("HTTP methods", methods, expectedNames(expectedMethods)); msg != "" {
			t.Error(msg)
		}
	})
}

// testSurfaceExtractor checks that the extraction finds the additions an
// update of index.js can make.
func testSurfaceExtractor(t *testing.T) {
	const js = `
// a comment mentioning ?commented and "?commentedq"
/** @param {string} x see ?docparam */
url.searchParams.set("newparam", "");
location.href = base + "?newq=" + encodeURIComponent(q);
const u = ` + "`${path}?first&second=1`" + `;
await fetch(u, { method: "COPY" });
xhr.open("PROPPATCH", url, true);
await checkAuth("newvariant");
new URLSearchParams({ ...PARAMS, newkey, other: item.name });
s.replace(/&/g, "&amp;").replace(/</g, "&lt;");
`
	queries, methods := extractSurface(js)
	assert.Equal(t, []string{"first", "newkey", "newparam", "newq", "newvariant", "other", "second"}, queries)
	assert.Equal(t, []string{"COPY", "PROPPATCH"}, methods)

	t.Run("Message", func(t *testing.T) {
		assert.Empty(t, surfaceDiffMessage("query parameters", []string{"a", "b"}, []string{"a", "b"}))
		msg := surfaceDiffMessage("query parameters", []string{"a", "new"}, []string{"a", "gone"})
		assert.Contains(t, msg, "new (not handled by the server?): [new]")
		assert.Contains(t, msg, "vanished (no longer used?): [gone]")
		assert.Contains(t, msg, "fork-doc/sviluppo.md")
		assert.Contains(t, msg, "Sincronizzazione")
	})
}
