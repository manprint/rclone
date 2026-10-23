package webui

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"golang.org/x/sync/singleflight"
)

// lookups runs the VFS calls which may wait on the remote, sharing each
// call between the requests which make it at the same time and letting
// every request stop waiting when its context is done.
//
// The VFS can't cancel a call, so one keeps running in the background
// after all its requests gave up. A remote which hangs then costs one call
// at a time rather than one per request. The zero value is ready to use.
type lookups struct {
	group singleflight.Group
}

// do returns the result of fn, sharing the call with the other callers
// with the same key, or the error of ctx if it is done first
func (l *lookups) do(ctx context.Context, key string, fn func() (any, error)) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ch := l.group.DoChan(key, func() (v any, err error) {
		// singleflight would re-panic in a goroutine of its own, taking
		// down the server rather than the request
		err = vfscommon.RecoverCall(key, func() (err error) {
			v, err = fn()
			return err
		})
		return v, err
	})
	select {
	case res := <-ch:
		return res.Val, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// stat is VFS.Stat which gives up when ctx is done
func (l *lookups) stat(ctx context.Context, VFS *vfs.VFS, remote string) (vfs.Node, error) {
	v, err := l.do(ctx, fmt.Sprintf("stat\x00%p\x00%s", VFS, remote), func() (any, error) {
		return VFS.Stat(remote)
	})
	if err != nil {
		return nil, err
	}
	return v.(vfs.Node), nil
}

// readDir is Dir.ReadDirAll which gives up when ctx is done
//
// The nodes returned may be shared with other callers so must not be
// modified.
func (l *lookups) readDir(ctx context.Context, dir *vfs.Dir) (vfs.Nodes, error) {
	v, err := l.do(ctx, fmt.Sprintf("readdir\x00%p", dir), func() (any, error) {
		return dir.ReadDirAll()
	})
	if err != nil {
		return nil, err
	}
	return v.(vfs.Nodes), nil
}
