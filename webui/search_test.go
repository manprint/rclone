package webui

import (
	"context"
	"testing"

	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchLimits checks the matching and the limits of a search
func TestSearchLimits(t *testing.T) {
	VFS, _ := newTestVFS(t, map[string]string{
		"a/b/c/d/match.txt": "deep",
		"match1.txt":        "1",
		"x/match2.txt":      "2",
		"x/other.txt":       "o",
		"UPPER/MATCH3.TXT":  "3",
		"plain.txt":         "p",
	})
	root, err := VFS.Root()
	require.NoError(t, err)
	newItem := func(entry vfs.Node, relDir string) PathItem {
		item := PathItem{PathType: PathTypeFile, Name: entry.Name()}
		if entry.IsDir() {
			item.PathType = PathTypeDir
		}
		if relDir != "" {
			item.Name = relDir + "/" + item.Name
		}
		return item
	}
	run := func(ctx context.Context, needle string, maxDepth, maxResults int) (out []string) {
		items, _ := search(ctx, &lookups{}, root, needle, maxDepth, maxResults, newItem)
		for _, item := range items {
			out = append(out, item.Name)
		}
		return out
	}
	ctx := context.Background()

	t.Run("Depth", func(t *testing.T) {
		assert.ElementsMatch(t, []string{"match1.txt", "x/match2.txt", "UPPER/MATCH3.TXT"}, run(ctx, "match", 1, 100))
		assert.ElementsMatch(t, []string{"match1.txt"}, run(ctx, "match", 0, 100))
		assert.ElementsMatch(t, []string{"match1.txt", "x/match2.txt", "UPPER/MATCH3.TXT", "a/b/c/d/match.txt"}, run(ctx, "match", 4, 100))
	})

	t.Run("MaxResults", func(t *testing.T) {
		assert.Len(t, run(ctx, "match", 8, 1), 1)
		assert.Len(t, run(ctx, "match", 8, 2), 2)
		assert.Empty(t, run(ctx, "match", 8, 0))
	})

	t.Run("BreadthFirst", func(t *testing.T) {
		assert.Equal(t, []string{"match1.txt"}, run(ctx, "match", 8, 1))
	})

	t.Run("ExpiredContext", func(t *testing.T) {
		expired, cancel := context.WithCancel(ctx)
		cancel()
		assert.Empty(t, run(expired, "match", 8, 100))
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		assert.Contains(t, run(ctx, "match", 8, 100), "UPPER/MATCH3.TXT")
		assert.Equal(t, []string{"UPPER"}, run(ctx, "upper", 8, 100))
	})

	t.Run("Directories", func(t *testing.T) {
		items, unread := search(ctx, &lookups{}, root, "b", 8, 100, newItem)
		assert.Zero(t, unread)
		require.Len(t, items, 1)
		assert.Equal(t, PathItem{PathType: PathTypeDir, Name: "a/b"}, items[0])
	})
}
