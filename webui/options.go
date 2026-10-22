// Package webui implements the superdufs web interface for rclone serve webdav
package webui

import (
	"time"

	"github.com/rclone/rclone/fs"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "webui",
	Default: true,
	Help:    "Serve the web interface to browsers listing a directory",
}, {
	Name:    "webui_allow_delete",
	Default: true,
	Help:    "Allow any client to delete, move or overwrite files while the web interface is on",
}, {
	Name:    "webui_search",
	Default: true,
	Help:    "Enable recursive search in the web interface",
}, {
	Name:    "webui_search_max_depth",
	Default: 8,
	Help:    "Maximum directory depth of a web interface search",
}, {
	Name:    "webui_search_max_results",
	Default: 1000,
	Help:    "Maximum number of results of a web interface search",
}, {
	Name:    "webui_search_timeout",
	Default: fs.Duration(30 * time.Second),
	Help:    "Maximum duration of a web interface search",
}, {
	Name:    "webui_publink_paths",
	Default: fs.CommaSepList{},
	Help:    "Paths below which public download links may be created",
}, {
	Name:      "webui_secret",
	Default:   "",
	Help:      "Secret used to sign web interface tokens",
	Sensitive: true,
}, {
	Name:    "webui_token_expiry",
	Default: fs.Duration(72 * time.Hour),
	Help:    "Lifetime of web interface download tokens",
}, {
	Name:    "webui_resume",
	Default: false,
	Help:    "Remember the playback position of media files",
}, {
	Name:    "webui_resume_dir",
	Default: "",
	Help:    "Directory where playback positions are stored",
}, {
	Name:    "webui_resume_retention",
	Default: fs.Duration(720 * time.Hour),
	Help:    "How long unused playback positions are kept",
}, {
	Name:    "webui_player_max_retry",
	Default: 30,
	Help:    "Maximum reconnection attempts of the web interface player",
}, {
	Name:    "webui_player_reconnect_interval",
	Default: 10,
	Help:    "Seconds between reconnection attempts of the web interface player",
}, {
	Name:    "webui_disk_stats",
	Default: false,
	Help:    "Show disk usage statistics in the web interface",
}, {
	Name:    "webui_upload_settings",
	Default: false,
	Help:    "Show the concurrent upload settings in the web interface",
}, {
	Name:    "webui_edit_max_size",
	Default: 4 * fs.Mebi,
	Help:    "Maximum size of a file editable in the web interface",
}, {
	Name:    "webui_dir_count",
	Default: false,
	Help:    "Show the number of entries of directories in the web interface",
}, {
	Name:    "webui_multi_zip_max",
	Default: 200,
	Help:    "Maximum number of items in a multiple selection zip download",
}}

// Options configures the web UI
type Options struct {
	Enabled                 bool            `config:"webui"`
	AllowDelete             bool            `config:"webui_allow_delete"`
	Search                  bool            `config:"webui_search"`
	SearchMaxDepth          int             `config:"webui_search_max_depth"`
	SearchMaxResults        int             `config:"webui_search_max_results"`
	SearchTimeout           fs.Duration     `config:"webui_search_timeout"`
	PublinkPaths            fs.CommaSepList `config:"webui_publink_paths"`
	Secret                  string          `config:"webui_secret"`
	TokenExpiry             fs.Duration     `config:"webui_token_expiry"`
	Resume                  bool            `config:"webui_resume"`
	ResumeDir               string          `config:"webui_resume_dir"`
	ResumeRetention         fs.Duration     `config:"webui_resume_retention"`
	PlayerMaxRetry          int             `config:"webui_player_max_retry"`
	PlayerReconnectInterval int             `config:"webui_player_reconnect_interval"`
	DiskStats               bool            `config:"webui_disk_stats"`
	UploadSettings          bool            `config:"webui_upload_settings"`
	EditMaxSize             fs.SizeSuffix   `config:"webui_edit_max_size"`
	DirCount                bool            `config:"webui_dir_count"`
	MultiZipMax             int             `config:"webui_multi_zip_max"`
}

// Opt is options set by command line flags
var Opt Options

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "webdav_webui", Opt: &Opt, Options: OptionsInfo})
}
