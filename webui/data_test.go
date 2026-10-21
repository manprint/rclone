package webui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNaturalLess(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		less bool
	}{
		{"a2", "a10", true},
		{"a10", "a2", false},
		{"file9", "File10", true},
		{"File10", "file9", false},
		{"b", "c", true},
		{"c", "b", false},
		{"", "a", true},
		{"a", "", false},
		{"", "", false},
		{"a", "a", false},
		{"A", "a", false},
		{"a", "a1", true},
		{"a1b2", "a1b10", true},
		{"a01", "a2", true},
		{"a2", "a01", false},
		// Same value: the shorter run sorts first, never both ways
		{"x1", "x01", true},
		{"x01", "x1", false},
		{"x01", "x01", false},
		// Runs longer than any integer
		{"n99999999999999999999", "n100000000000000000000", true},
		{"1a", "a1", true},
		{"é", "z", false},
	} {
		assert.Equal(t, tc.less, naturalLess(tc.a, tc.b), "%q < %q", tc.a, tc.b)
	}
}

func TestSortPathItems(t *testing.T) {
	items := func() []PathItem {
		return []PathItem{
			{PathType: PathTypeFile, Name: "b.txt", Mtime: 300, Size: 5},
			{PathType: PathTypeFile, Name: "a10.txt", Mtime: 100, Size: 30},
			{PathType: PathTypeDir, Name: "zdir", Mtime: 200, Size: -1},
			{PathType: PathTypeFile, Name: "a2.txt", Mtime: 200, Size: 10},
			{PathType: PathTypeDir, Name: "Adir", Mtime: 400, Size: -1},
		}
	}
	names := func(items []PathItem) []string {
		out := make([]string, len(items))
		for i := range items {
			out[i] = items[i].Name
		}
		return out
	}
	for _, tc := range []struct {
		name    string
		sortBy  string
		order   string
		hasSort bool
		want    []string
	}{
		{"Default", "", "", false, []string{"Adir", "zdir", "a2.txt", "a10.txt", "b.txt"}},
		{"ByName", "name", "", true, []string{"Adir", "zdir", "a2.txt", "a10.txt", "b.txt"}},
		{"ByNameDesc", "name", "desc", true, []string{"b.txt", "a10.txt", "a2.txt", "zdir", "Adir"}},
		{"ByNameAsc", "name", "asc", true, []string{"Adir", "zdir", "a2.txt", "a10.txt", "b.txt"}},
		{"ByMtime", "mtime", "", true, []string{"zdir", "Adir", "a10.txt", "a2.txt", "b.txt"}},
		{"ByMtimeDesc", "mtime", "desc", true, []string{"b.txt", "a2.txt", "a10.txt", "Adir", "zdir"}},
		// Equal sizes keep the input order
		{"BySize", "size", "", true, []string{"zdir", "Adir", "b.txt", "a2.txt", "a10.txt"}},
		{"BySizeDesc", "size", "desc", true, []string{"a10.txt", "a2.txt", "b.txt", "Adir", "zdir"}},
		{"DescWithoutSortIgnored", "", "desc", false, []string{"Adir", "zdir", "a2.txt", "a10.txt", "b.txt"}},
		{"UnknownSortIsName", "bogus", "", true, []string{"Adir", "zdir", "a2.txt", "a10.txt", "b.txt"}},
		{"UnknownSortDesc", "bogus", "desc", true, []string{"b.txt", "a10.txt", "a2.txt", "zdir", "Adir"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := items()
			sortPathItems(got, tc.sortBy, tc.order, tc.hasSort)
			assert.Equal(t, tc.want, names(got))
		})
	}

	t.Run("Empty", func(t *testing.T) {
		sortPathItems(nil, "size", "desc", true)
	})
}
