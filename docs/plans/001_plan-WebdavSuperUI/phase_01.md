# Phase 0 — Fondamenta e listing

Intent: con `rclone serve webdav <dir>` un browser vede la pagina superdufs con il listing reale della VFS
(`GET dir/` HTML, `?json`, ordinamento), asset serviti, flag `--webui*` registrati; con UI disattivata il
comportamento è identico a upstream.
Prerequisites: none.
Phase closure: P0; review by Opus (agent-1).

## State and ownership contract
Read STATE.md §0–1 first for scope, ownership, recovery, checks, and commit rules.
Open before edits; checkpoint at a recovery boundary; close with evidence. A
delegated worker follows its assigned OPEN unit. Missing design goes to the
configured supervisor.

## Local design context
Plan revision: 1.
- D2: unici edit a file upstream = 3 righe aggiunte in `cmd/serve/webdav/webdav.go`, ognuna termina con `// fork:webui`. Nessuna riga esistente toccata (attenzione a gofmt: non deve riallineare righe esistenti). Niente `go.mod`/`go.sum`.
- D3: package `webui` in `webui/` (import `github.com/rclone/rclone/webui`); glue in `cmd/serve/webdav/webdav_webui.go`.
- D4: asset superdufs copiati byte-identici da `<superdufs>/assets/` (commit `347d202`): `index.html`, `index.js`, `index.css`, `favicon.ico`. Overlay fork: `webui/overlay/overlay.js`.
- D5/D6: opzioni globali blocco `webdav_webui`; UI attiva sse `Opt.Enabled && opt.Template.Path == "" && !opt.DisableDirList`.
- D7: `PreAuth` = primo middleware (via `libhttp.Option`), `Handler` = dopo i middleware upstream, prima di `router.Handle`. Tutto ciò che non è gestito → `next.ServeHTTP`.
- D8: pagina UI solo se `wantsUI(r)`: header `Accept` contiene `text/html` E User-Agent (lowercase) NON inizia con uno di `lynx/`, `w3m/`, `links `, `elinks/`, `curl/`, `wget/`, `httpie/`, `aria2/`, `rclone/`. Altrimenti `next` (listing upstream).
- D11: `size` cartelle = `-1`; con `--webui-dir-count` = numero figli (`ReadDirAll`, cap 1000; errore → -1).
- D15 (parte pagine): risposte HTML UI con `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-cache`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: frame-ancestors 'self'; object-src 'none'; base-uri 'self'`.
- I-1, I-2, I-3, I-7: vedi overview; guardati da G-PHASE (doppio run env), `TestWebUIDisabledByTemplate`, `TestWebUIDisabledFlag`, G-FOOT, `TestAssetsMatchSource`.
- R2: le `libhttp.Option` sono applicate in `NewServer` prima di ogni `s.mux.Use`; chi va in panic se `Use` è chiamato dopo una rotta → `webuiUse` va prima di `router.Handle("/*", w)`.

### Contratto dati (JSON, nomi esatti)
```go
type PathType string // "Dir" | "File"  (rclone non espone symlink)
type PathItem struct {
	PathType       PathType `json:"path_type"`
	Name           string   `json:"name"`            // leaf (o path relativo nella ricerca)
	Mtime          int64    `json:"mtime"`           // millisecondi Unix; 0 se VFS.Opt.NoModTime
	Size           int64    `json:"size"`            // file: byte; dir: -1 o conteggio (D11)
	ResumePosition *int64   `json:"resume_position,omitempty"` // fase 1.3
	PublinkToken   *string  `json:"publink_token,omitempty"`   // fase 3.2
}
type IndexData struct {
	Href                          string     `json:"href"`        // "/" + dirRemote (root: "/"), senza slash finale, senza baseurl
	Kind                          string     `json:"kind"`        // "Index"
	URIPrefix                     string     `json:"uri_prefix"`  // baseSlash: "/" oppure "/prefix/"
	AllowUpload                   bool       `json:"allow_upload"`  // !VFS.Opt.ReadOnly
	AllowDelete                   bool       `json:"allow_delete"`  // !VFS.Opt.ReadOnly
	AllowSearch                   bool       `json:"allow_search"`  // Opt.Search
	AllowArchive                  bool       `json:"allow_archive"` // !cfg.DisableZip
	DirExists                     bool       `json:"dir_exists"`    // true
	Auth                          bool       `json:"auth"`          // cfg.UsingAuth()
	User                          *string    `json:"user"`          // null se nessun utente
	Paths                         []PathItem `json:"paths"`         // mai null: [] se vuota
	PlaybackResumeEnabled         bool       `json:"playback_resume_enabled"` // fase 1.3 (false ora)
	PublinkEnabled                bool       `json:"publink_enabled"`         // fase 3.2 (false ora)
	PublinkIsViewer               bool       `json:"publink_is_viewer"`       // false
	EnableDataStats               bool       `json:"enable_data_stats"`       // Opt.DiskStats
	ShowConcurrentUploadsSettings bool       `json:"show_concurrent_uploads_settings"` // Opt.UploadSettings
	DiskUsage                     *int64     `json:"disk_usage"`     // sempre null (caricati via ?disk-stats)
	DiskTotal                     *int64     `json:"disk_total"`
	DiskAvailable                 *int64     `json:"disk_available"`
}
```
Utente: `libhttp.CtxGetUser(ctx)`; se assente e `cfg.UsingAuth()` → `r.BasicAuth()` (l'auth-proxy non imposta l'utente nel contesto).

### Ordinamento (identico a superdufs `sort_path_items`)
- Chiave primaria sempre: cartelle prima dei file.
- `sort=mtime` → per Mtime crescente; `sort=size` → per Size crescente; altrimenti (assente, `name`, sconosciuto) → nome naturale.
- Nome naturale = confronto case-insensitive (lowercase) a blocchi: sequenze di cifre confrontate numericamente (ignorando zeri iniziali; a parità, la più corta prima), il resto rune per rune. Esempi: `a2 < a10`, `file9 < File10`, `b < c`.
- Solo se `sort` è presente e `order=desc` → inverti l'intera slice (le cartelle finiscono in fondo: è il comportamento superdufs).
- Usa `sort.SliceStable`.

### Render HTML
`New` gira dentro `libhttp.NewServer`, prima che `newWebDAV` normalizzi `w.opt.HTTP.BaseURL` (webdav.go: la normalizzazione segue `NewServer`): il template non deve dipendere dal baseurl al momento di `New`.
1. In `New`, sul template grezzo: `tag := "<script src=\"__ASSETS_PREFIX__index.js\"></script>"`; se `strings.Count(indexHTML, tag) != 1` → errore (asset incompatibili); `rawTpl = strings.Replace(indexHTML, tag, tag + "\n  <script src=\"__ASSETS_PREFIX__overlay.js\"></script>", 1)`. (Verificato: `index.html` riga 260 contiene esattamente `<script src="__ASSETS_PREFIX__index.js"></script>`, senza `defer`.)
2. Per ogni richiesta: `prefixURL = u.baseSlash() + assetsPrefix` (es. `/__webui_1a2b3c4d__/`); `html = strings.ReplaceAll(rawTpl, "__ASSETS_PREFIX__", prefixURL)`; poi `strings.Replace(html, "__INDEX_DATA__", base64.StdEncoding.EncodeToString(jsonBytes), 1)`.
3. `baseSlash()` tollera input non normalizzato: `b := strings.Trim(cfg.BaseURL(), "/")`; `b == ""` → `"/"`; altrimenti `"/" + b + "/"`. Stessa normalizzazione ovunque serva il prefisso senza slash finale (`strings.TrimSuffix(baseSlash(), "/")`).

### Asset
`assetsPrefix = "__webui_" + hex(sha256(index.html‖index.js‖index.css‖favicon.ico‖overlay.js))[:8] + "__/"`.
Nomi serviti: `index.js` (`application/javascript; charset=utf-8`), `index.css` (`text/css; charset=utf-8`), `favicon.ico` (`image/x-icon`), `overlay.js` (`application/javascript; charset=utf-8`). Header: `ETag: "<hex sha256 del file[:16]>"`, `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`. `If-None-Match` uguale → 304 senza body. Metodi GET/HEAD; altri → 405. Nome sconosciuto → 404. Serviti da `PreAuth` (senza auth: servono anche alle pagine pubbliche).

## Sub-phases

### 0.1 Scaffolding: package, opzioni, asset, glue, script
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P0 (nessuna review focalizzata).
- **Files:**
  READ `cmd/serve/webdav/webdav.go` (struct `WebDAV`, `newWebDAV`, `checkPath`, `getVFS`), `lib/http/server.go` (`Option`, `Router`, `UsingAuth`), `lib/http/middleware.go` (`MiddlewareCORS`, `MiddlewareResponseHeaders`), `fs/registry.go` (`Option`, `Options`, `OptionsInfo`, `RegisterGlobalOptions`), `fs/config/flags` (`AddFlagsFromOptions`), `cmd/serve/proxy/proxy.go` (`HoldVFS`, `IsProxy`), un package con opzioni nuovo stile (es. `cmd/serve/webdav/webdav.go` `OptionsInfo`).
  WRITE `cmd/serve/webdav/webdav.go` (solo le 3 righe).
  NEW `webui/options.go`, `webui/ui.go`, `webui/assets.go`, `webui/assets/{index.html,index.js,index.css,favicon.ico,SOURCE}`, `webui/overlay/overlay.js`, `webui/ui_test.go`, `cmd/serve/webdav/webdav_webui.go`, `cmd/serve/webdav/webdav_webui_test.go`, `webui/scripts/check-footprint.sh`, `webui/scripts/sync-assets.sh`, `webui/README.md`.
- **Change:**
  Preconditions: baseline `81e7372f6`, G-BASE verde.
  Contract:
  - `webui/options.go`:
    ```go
    // Options configures the web UI
    type Options struct {
        Enabled                 bool             `config:"webui"`
        Search                  bool             `config:"webui_search"`
        SearchMaxDepth          int              `config:"webui_search_max_depth"`
        SearchMaxResults        int              `config:"webui_search_max_results"`
        SearchTimeout           fs.Duration      `config:"webui_search_timeout"`
        PublinkPaths            fs.CommaSepList  `config:"webui_publink_paths"`
        Secret                  string           `config:"webui_secret"`
        TokenExpiry             fs.Duration      `config:"webui_token_expiry"`
        Resume                  bool             `config:"webui_resume"`
        ResumeDir               string           `config:"webui_resume_dir"`
        ResumeRetention         fs.Duration      `config:"webui_resume_retention"`
        PlayerMaxRetry          int              `config:"webui_player_max_retry"`
        PlayerReconnectInterval int              `config:"webui_player_reconnect_interval"`
        DiskStats               bool             `config:"webui_disk_stats"`
        UploadSettings          bool             `config:"webui_upload_settings"`
        EditMaxSize             fs.SizeSuffix    `config:"webui_edit_max_size"`
        DirCount                bool             `config:"webui_dir_count"`
        MultiZipMax             int              `config:"webui_multi_zip_max"`
    }
    ```
    `OptionsInfo fs.Options` con un `fs.Option` per campo (Name = tag, Default come tabella overview: true, true, 8, 1000, `fs.Duration(30*time.Second)`, `fs.CommaSepList{}`, "", `fs.Duration(72*time.Hour)`, false, "", `fs.Duration(720*time.Hour)`, 30, 10, false, false, `4*fs.Mebi`, false, 200; `Secret` con `Sensitive: true`). Help: una frase inglese per flag. `var Opt Options`; `init()` chiama `fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "webdav_webui", Opt: &Opt, Options: OptionsInfo})`.
  - `webui/ui.go`:
    ```go
    // Config connects the web UI to the server it is attached to
    type Config struct {
        BaseURL         func() string                                   // "", "/" o "/prefix" letto a richiesta
        GetVFS          func(ctx context.Context) (*vfs.VFS, error)
        HoldVFS         func(http.Handler) http.Handler
        CheckPath       func(name string) error
        UsingAuth       func() bool
        Realm           string
        DisableZip      bool
        AllowOrigin     string
        ResponseHeaders []string
        IsProxy         bool
    }
    // UI is a web UI attached to a server
    type UI struct { cfg Config; opt Options; publicWrap func(http.Handler) http.Handler; /* campi futuri */ }
    func New(cfg Config, opt Options) (*UI, error)        // valida overlay injection, fs.ParseHeaders, costruisce publicWrap = CORS(RespHeaders(h))
    func (u *UI) PreAuth(next http.Handler) http.Handler   // 0.1: solo asset; resto → next
    func (u *UI) Handler(next http.Handler) http.Handler   // 0.1: pass-through puro
    func (u *UI) baseSlash() string                        // "/" o "/prefix/" da cfg.BaseURL()
    ```
    PreAuth: calcola il path relativo togliendo `strings.TrimSuffix(baseSlash,"/")` (se baseurl non è prefisso → next); se il relativo inizia con `"/"+assetsPrefix` → `u.publicWrap(assetHandler)`.
  - `cmd/serve/webdav/webdav_webui.go` (header `// fork:webui` come prima riga di commento del file, godoc in inglese):
    ```go
    func init() { flags.AddFlagsFromOptions(Command.Flags(), "", webui.OptionsInfo) }
    // webuiGlue is the web UI attached to a WebDAV server
    type webuiGlue = webui.UI
    func (w *WebDAV) webuiPreAuth() libhttp.Option // closure: se UI attiva (D6) → webui.New(cfg, webui.Opt) ; errore → fs.Errorf(nil, "webui disabled: %v", err) e return; w.webui = u; s.Router().Use(u.PreAuth)
    func (w *WebDAV) webuiUse(router chi.Router)    // se w.webui != nil → router.Use(w.webui.Handler)
    ```
    Config: `BaseURL: func() string { return w.opt.HTTP.BaseURL }`, `GetVFS: w.getVFS`, `HoldVFS: w.provider.HoldVFS`, `CheckPath: checkPath`, `UsingAuth: s.UsingAuth`, `Realm: w.opt.Auth.Realm`, `DisableZip: w.opt.DisableZip`, `AllowOrigin: w.opt.HTTP.AllowOrigin`, `ResponseHeaders: w.opt.HTTP.ResponseHeaders`, `IsProxy: w.provider.IsProxy()`.
  - Le 3 righe in `webdav.go`: (a) ultimo campo di `WebDAV` dopo `etagHashType`: `webui         *webuiGlue // fork:webui`; (b) in `libhttp.NewServer(ctx, …)` dopo `libhttp.WithTemplate(w.opt.Template),`: `w.webuiPreAuth(), // fork:webui`; (c) subito prima di `router.Handle("/*", w)`: `w.webuiUse(router) // fork:webui`.
  - `check-footprint.sh` (bash, `set -euo pipefail`, eseguibile, cwd = root repo): `BASE=${UPSTREAM_REF:-}`; se vuoto: usa `upstream/master` se il remote `upstream` esiste, altrimenti `origin/master`; `MB=$(git merge-base HEAD "$BASE")`. Fallisce (exit 1, messaggio con path) se `git diff --name-only "$MB"` (include working tree) contiene file fuori da: `^webui/`, `^docs/plans/`, `^cmd/serve/webdav/[^/]*_webui[^/]*\.go$`, `^cmd/serve/webdav/webdav\.go$`. Per `webdav.go`: `git diff -U0 "$MB" -- cmd/serve/webdav/webdav.go` → nessuna riga `^-[^-]`, ogni riga `^\+[^+]` contiene `fork:webui`, al massimo 3 righe aggiunte. Stampa `footprint OK` se passa.
  - `sync-assets.sh <superdufs-dir>`: copia i 4 asset in `webui/assets/`, scrive `SOURCE` (`repo: <git remote get-url origin>` con l'eventuale userinfo rimosso — `sed -E 's#(://)[^/@]+@#\1#'` — per non committare credenziali, `commit: <git rev-parse HEAD>`, poi una riga `sha256 <name> <hex>` per file, ordine fisso index.html, index.js, index.css, favicon.ico), poi lancia `go test ./webui/ -run 'TestAssets|TestAPISurface' -count=1`.
  - `overlay.js`: ridefinisce `function formatDirSizeParts(size)`: se `size == null || size < 0` → `["—", ""]`; altrimenti identica all'originale (`index.js` riga ~2640: unit `item`/`items`, cap `MAX_SUBPATHS_COUNT`). Prima di scrivere, verifica dove viene usato il risultato e che `["—",""]` renda "—".
  - `webui/README.md`: titolo, scopo, sezione "Flag" (tabella dei flag), sezione "Sincronizzazione" (script), in italiano. Le fasi successive aggiungono sezioni.
  Scope: nessuna logica di listing (0.2). `Handler` resta pass-through.
  Steps:
  1. S1 — Copia asset + `SOURCE` con `sync-assets.sh` (prima scrivi lo script; il test lanciato alla fine può fallire finché S4 non esiste: esegui lo script con la riga `go test` commentata temporaneamente oppure ignora l'esito in questo passo); expected: 4 file identici (cmp) a superdufs.
  2. S2 — `options.go`, `assets.go` (`//go:embed assets/index.html assets/index.js assets/index.css assets/favicon.ico overlay/overlay.js`, calcolo prefisso/ETag, handler), `ui.go`; expected: `go build ./webui/` ok.
  3. S3 — glue + 3 righe in `webdav.go`; `gofmt -l cmd/serve/webdav webui` vuoto; expected: `go build ./...` ok; `git diff cmd/serve/webdav/webdav.go` = 3 righe `+` soltanto.
  4. S4 — test (sotto), `check-footprint.sh`, README; expected: G-U01 verde, G-FOOT `footprint OK`.
  Recovery boundary: dopo S3 (build verde) se la sessione sta per terminare.
  Failure handling: gofmt riallinea righe esistenti → sposta la riga aggiunta finché il diff resta di 3 righe `+`; impossibile → escalation Opus. `AddFlagsFromOptions` panic per nome duplicato → escalation.
- **Unit tests:** in `webui/ui_test.go`:
  `TestAssetsMatchSource` (sha256 degli asset embedded == righe di `SOURCE`); `TestAssetsServed` (httptest: `New` con Config minimale e BaseURL `/`; GET `/<prefix>index.js` → 200, Content-Type js, ETag presente; stessa GET con `If-None-Match` = ETag → 304; GET `/<prefix>nope.js` → 404; POST → 405; con BaseURL `/base` il path `/base/<prefix>index.css` → 200 e `/<prefix>index.css` → passa a `next`); `TestOverlayInjected` (template contiene `overlay.js` dopo `index.js`).
  In `cmd/serve/webdav/webdav_webui_test.go` (stesso build tag di `webdav_test.go`: `//go:build !windows && !darwin`, perché usa le sue costanti): `func init()` che, se è impostata la variabile `RCLONE_WEBUI`, assegna `webui.Opt.Enabled` con `strconv.ParseBool` (nei test `fs.GlobalOptionsInit` non gira, quindi l'env non arriva a `webui.Opt`: senza questo init il doppio run di G-PHASE con `RCLONE_WEBUI=false` non disattiverebbe la UI); helper `startWebUI(t, mutate func(*Options), uiMutate func(*webui.Options), vfsOpt *vfscommon.Options) (url string)` che salva/ripristina `webui.Opt` (`t.Cleanup`), forza `webui.Opt.Enabled=true` prima di `uiMutate`, usa `fs.NewFs(ctx, t.TempDir())` e `opt.HTTP.ListenAddr = []string{testBindAddress}`; `TestWebUIFlagsRegistered` (`Command.Flags().Lookup("webui")`, `"webui-search-max-depth"`, `"webui-secret"` non nil; default `webui` = "true"); `TestWebUIAssetsReachable` (GET asset via server reale → 200).
  Gate: G-U01. Discovery: `-v` mostra i 5 test RUN/PASS.
- **e2e tests:** N/A — nessun comportamento utente nuovo oltre agli asset; l'e2e del listing è in 0.2.
- **Done:** asset+SOURCE identici, glue compilato, 3 righe in webdav.go, G-U01 e G-FOOT verdi, G-BASE ancora verde (`go test ./cmd/serve/webdav/`), unit chiusa in STATE.md, commit `sub-phase(0.1)`.

### 0.2 Listing, pagina Index, JSON, redirect cartelle
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P0.
- **Files:**
  READ `webui/*.go` (0.1), `vfs/vfs.go` (`Stat`, `Node`), `vfs/dir.go` (`ReadDirAll`), `lib/http/serve/dir.go` (`Error`), `lib/http/context.go` (`CtxGetUser`), `cmd/serve/webdav/webdav_test.go` (`testBindAddress`, `testTemplate`, helper esistenti), `<superdufs>/src/server.rs` `sort_path_items`/`sort_by_*` (riferimento).
  WRITE `webui/ui.go` (`Handler`), `webui/README.md`, `cmd/serve/webdav/webdav_webui_test.go`.
  NEW `webui/data.go` (tipi + sort + natural compare), `webui/listing.go` (build/serve listing), `webui/data_test.go`.
- **Change:**
  Preconditions: 0.1 DONE.
  Contract — `Handler(next)` per GET/HEAD (altri metodi → next in questa sub-fase):
  1. `path := r.URL.Path`; se `cfg.CheckPath(path) != nil` → next.
  2. `remote := strings.Trim(path, "/")`; `VFS, err := cfg.GetVFS(ctx)`; err → next. `node, err := VFS.Stat(remote)`; err (anche ENOENT) → next.
  3. Se `!node.IsDir()` → next (file: fasi successive).
  4. Cartella senza slash finale (`path != "/" && !strings.HasSuffix(path, "/")`): se GET e `wantsUI(r)` → `302` con `Location: strings.TrimSuffix(baseSlash,"/") + path + "/"` + (`"?"+r.URL.RawQuery` se non vuota); altrimenti next.
  5. Cartella con slash, query: se presente `download` → next (zip upstream). Se presente `json` → JSON. Altrimenti se `wantsUI(r)` → HTML. Altrimenti → next. (Le query `zip`, `q`, `disk-stats`, `tokengen` arrivano nelle fasi 2/3: per ora ignorate = trattate come listing.)
  6. Listing: `dir := node.(*vfs.Dir)`; `items, err := dir.ReadDirAll()`; err → `serve.Error(ctx, remote, rw, "Failed to list directory", err)`. Per ogni nodo → `PathItem` (Contratto dati, D11). `sortPathItems(items, query.Get("sort"), query.Get("order"), query.Has("sort"))`.
  7. JSON: `Content-Type: application/json`, `Cache-Control: no-cache`, body `json.Marshal(IndexData)`. HTML: header D15, body da Render HTML. HEAD → solo header (stesso Content-Type), nessun body.
  Scope: nessun cambiamento a richieste non-GET/HEAD; nessuna gestione file.
  Steps:
  1. S1 — `data.go`: tipi, `naturalLess(a, b string) bool`, `sortPathItems`; test unitari; expected G-U02a verde.
  2. S2 — `listing.go` + `Handler` (punti 1–7), `wantsUI`, utente; expected build ok.
  3. S3 — test integrazione in `webdav_webui_test.go`; README sezione "Interfaccia web" (cosa appare, fallback CLI, flag che la disattivano); expected G-U02b verde, G-BASE verde.
  Recovery boundary: none.
  Failure handling: un test upstream fallisce con UI attiva → non modificarlo; escalation Opus con nome test e output.
- **Unit tests:** `webui/data_test.go`: `TestNaturalLess` (tabella: `a2<a10`, `file9<File10`, `b<c`, `x01==x1`→ false entrambe le direzioni o ordine stabile, `""<"a"`); `TestSortPathItems` (cartelle prima con sort name/mtime/size; `order=desc` con `sort` inverte tutto; `order=desc` senza `sort` ignorato; sort sconosciuto → nome). Gate G-U02a.
- **e2e tests:** `cmd/serve/webdav/webdav_webui_test.go` (server reale su TempDir con `dir1/`, `a10.txt`, `a2.txt`, file `x.txt` 3 byte), gate G-U02b:
  - `TestWebUIListingHTML`: GET `/` con `Accept: text/html` e UA `Mozilla/5.0` → 200, Content-Type html, header D15, body contiene `overlay.js` e `<template id="index-data">`; decodifica base64 dentro il template → IndexData con `kind=="Index"`, `href=="/"`, `uri_prefix=="/"`, `paths[0].name=="dir1"` con `path_type=="Dir"` e `size==-1`, poi `a2.txt`, `a10.txt`; `allow_upload==true`, `auth==false`, `user==null`.
  - `TestWebUIListingJSON`: GET `/?json` (senza Accept) → JSON; `GET /?json&sort=size&order=desc` → ordine invertito; `mtime` in ms (> 1e12).
  - `TestWebUICLIFallback`: GET `/` con UA `curl/8.0` (Accept `*/*`) → body contiene il listing upstream (`<title>` del template upstream "Directory listing of /"), non `index-data`.
  - `TestWebUIDirRedirect`: GET `/dir1` con Accept html → 302 `Location: /dir1/`; con baseurl `/pfx` → `Location: /pfx/dir1/`; GET `/dir1` con UA curl → 405 (upstream).
  - `TestWebUIReadOnly`: VFS `ReadOnly=true` → `allow_upload==false && allow_delete==false`.
  - `TestWebUIDirCount`: `uiMutate` DirCount=true → `dir1` con 2 file → `size==2`.
  - `TestWebUIDisabledByTemplate`: `opt.Template.Path = testTemplate` → GET `/` html → listing upstream (nessun `index-data`).
  - `TestWebUIDisabledFlag`: `webui.Opt.Enabled=false` (impostato dopo il force del helper) → idem.
  - `TestWebUIAuthUser`: `opt.Auth.BasicUser/Pass` → GET `/?json` con basic auth → `auth==true`, `user=="user"`.
  - `TestWebUIUpstreamSuiteEnabled` (I-2): `TestWebDav` e `TestHTTPFunction` impostano `Template.Path`, quindi con D6 girano a UI spenta. Questo test copia la funzione `start` di `TestWebDav` (leggerla prima) senza `Template.Path`, forza `webui.Opt.Enabled=true` (ripristino con `t.Cleanup`), verifica con un `require` che `w.webui != nil`, ed esegue `servetest.Run(t, "webdav", start)`: il client WebDAV reale di rclone (backend `webdav`, vendor rclone, nessun `Origin`) attraversa i middleware webui.
- **Done:** RS parte "pagina e JSON" soddisfatta; G-U02a, G-U02b verdi; G-BASE verde; README aggiornato; unit chiusa; commit `sub-phase(0.2)`.

## Phase gates and closure
- Required gates: G-PHASE (include doppio run upstream con `RCLONE_WEBUI=false`), G-FOOT, G-LINT.
- README obligation: `webui/README.md` sezioni "Flag", "Sincronizzazione", "Interfaccia web" coerenti con il comportamento.
- Every sub-phase DONE. Run full phase gates now.
- Supervisor Opus reviews integration (ordine middleware, 3 righe), I-1, I-2, I-3, I-7, test, README.
- Open closure unit P0, record review and result, mark phase DONE, commit `phase(0)`.
