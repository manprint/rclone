package webui

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// PathType is the kind of a listed entry
type PathType string

// The kinds of listed entries. rclone doesn't expose symlinks.
const (
	PathTypeDir  PathType = "Dir"
	PathTypeFile PathType = "File"
)

// PathItem is one entry of a directory listing as the page expects it
type PathItem struct {
	PathType       PathType `json:"path_type"`
	Name           string   `json:"name"`  // leaf name
	Mtime          int64    `json:"mtime"` // Unix milliseconds, 0 if the VFS doesn't read modification times
	Size           int64    `json:"size"`  // bytes for files, -1 or the number of children for directories
	ResumePosition *int64   `json:"resume_position,omitempty"`
	PublinkToken   *string  `json:"publink_token,omitempty"`
}

// IndexData is the data embedded in the page, also returned by ?json
type IndexData struct {
	Href                          string     `json:"href"`       // "/" + directory, without trailing slash or base URL
	Kind                          string     `json:"kind"`       // "Index"
	URIPrefix                     string     `json:"uri_prefix"` // base URL with leading and trailing slash
	AllowUpload                   bool       `json:"allow_upload"`
	AllowDelete                   bool       `json:"allow_delete"`
	AllowSearch                   bool       `json:"allow_search"`
	AllowArchive                  bool       `json:"allow_archive"`
	DirExists                     bool       `json:"dir_exists"`
	Auth                          bool       `json:"auth"`
	User                          *string    `json:"user"`
	Paths                         []PathItem `json:"paths"`
	PlaybackResumeEnabled         bool       `json:"playback_resume_enabled"`
	PublinkEnabled                bool       `json:"publink_enabled"`
	PublinkIsViewer               bool       `json:"publink_is_viewer"`
	EnableDataStats               bool       `json:"enable_data_stats"`
	ShowConcurrentUploadsSettings bool       `json:"show_concurrent_uploads_settings"`
	DiskUsage                     *int64     `json:"disk_usage"`
	DiskTotal                     *int64     `json:"disk_total"`
	DiskAvailable                 *int64     `json:"disk_available"`
}

// isDigit reports whether b is an ASCII digit
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// digitRun returns the length of the run of ASCII digits at the start of s
func digitRun(s string) int {
	n := 0
	for n < len(s) && isDigit(s[n]) {
		n++
	}
	return n
}

// compareNumeric compares two runs of digits by value without parsing
// them, so any length is handled
func compareNumeric(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	switch {
	case len(a) != len(b):
		return len(a) - len(b)
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// naturalCompare compares a and b as already lowercased strings, treating
// runs of digits as numbers: it returns <0, 0 or >0.
//
// Runs with the same value but different lengths ("1" and "01") compare
// equal there, but the shorter one sorts first if nothing else differs.
func naturalCompare(a, b string) int {
	tie := 0
	for len(a) > 0 && len(b) > 0 {
		if isDigit(a[0]) && isDigit(b[0]) {
			na, nb := digitRun(a), digitRun(b)
			if c := compareNumeric(a[:na], b[:nb]); c != 0 {
				return c
			}
			if tie == 0 {
				tie = na - nb
			}
			a, b = a[na:], b[nb:]
			continue
		}
		ra, sa := utf8.DecodeRuneInString(a)
		rb, sb := utf8.DecodeRuneInString(b)
		if ra != rb {
			if ra < rb {
				return -1
			}
			return 1
		}
		a, b = a[sa:], b[sb:]
	}
	switch {
	case len(a) != len(b):
		return len(a) - len(b)
	case tie < 0:
		return -1
	case tie > 0:
		return 1
	}
	return 0
}

// naturalLess reports whether a sorts before b ignoring case and
// comparing runs of digits by value ("a2" < "a10")
func naturalLess(a, b string) bool {
	return naturalCompare(strings.ToLower(a), strings.ToLower(b)) < 0
}

// sortPathItems sorts items for display.
//
// Directories come first. Within each kind the sort is by modification
// time or size if sortBy is "mtime" or "size" and by natural name order
// for anything else. If hasSort is set, which means the request asked for
// a sort, and order is "desc" the whole list is reversed, directories
// included.
func sortPathItems(items []PathItem, sortBy, order string, hasSort bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := &items[i], &items[j]
		if (a.PathType == PathTypeDir) != (b.PathType == PathTypeDir) {
			return a.PathType == PathTypeDir
		}
		switch sortBy {
		case "mtime":
			return a.Mtime < b.Mtime
		case "size":
			return a.Size < b.Size
		}
		return naturalLess(a.Name, b.Name)
	})
	if hasSort && order == "desc" {
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}
	}
}
