// fork:webui

package webdav

import (
	chi "github.com/go-chi/chi/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/webui"
)

func init() { flags.AddFlagsFromOptions(Command.Flags(), "", webui.OptionsInfo) }

// webuiGlue is the web UI attached to a WebDAV server
type webuiGlue = webui.UI

// webuiPreAuth returns the server option which attaches the web UI to the
// server and installs its middleware ahead of authentication.
//
// The UI is left off when the user asked for a template or no directory
// listing, so the server then behaves exactly as without the UI.
func (w *WebDAV) webuiPreAuth() libhttp.Option {
	return func(s *libhttp.Server) {
		if !webui.Opt.Enabled || w.opt.Template.Path != "" || w.opt.DisableDirList {
			return
		}
		u, err := webui.New(webui.Config{
			BaseURL:         func() string { return w.opt.HTTP.BaseURL },
			GetVFS:          w.getVFS,
			HoldVFS:         w.provider.HoldVFS,
			CheckPath:       checkPath,
			UsingAuth:       s.UsingAuth,
			Realm:           w.opt.Auth.Realm,
			DisableZip:      w.opt.DisableZip,
			AllowOrigin:     w.opt.HTTP.AllowOrigin,
			ResponseHeaders: w.opt.HTTP.ResponseHeaders,
			IsProxy:         w.provider.IsProxy(),
		}, webui.Opt)
		if err != nil {
			fs.Errorf(nil, "webui disabled: %v", err)
			return
		}
		w.webui = u
		s.Router().Use(u.PreAuth)
	}
}

// webuiUse adds the web UI middleware, if enabled, to the router.
//
// It must be called after the other middleware and before the routes are
// registered as chi panics if Use is called after a route.
func (w *WebDAV) webuiUse(router chi.Router) {
	if w.webui != nil {
		router.Use(w.webui.Handler)
	}
}
