# Fork di rclone: frontend superdufs per `rclone serve webdav`

> Documento storico: è l'analisi scritta prima dello sviluppo, il 21 ottobre 2026, conservata com'era salvo
> la rimozione di un indirizzo privato. Dove differisce da quanto realizzato (lo zip non usa
> `vfs.CreateZip`, vedi la revisione 1.1 del piano; il fork vive sul ramo `main` di
> https://github.com/manprint/rclone; la CI del fork) valgono la [documentazione](../README.md) e lo
> [stato del piano](../piano/001_plan-WebdavSuperUI/STATE.md).

Analisi tecnica · 21 ottobre 2026
Base upstream analizzata: `rclone/rclone` @ `81e7372f6` (20 ott 2026), branch locale `feat/webdav-frontend`
Frontend di riferimento: `superdufs` @ `347d202` (`assets/index.{html,js,css}`, `src/server.rs`)

---

## 1. Sintesi

**Si può fare, e in modo che i merge da upstream restino quasi a costo zero.** Il frontend di superdufs è un'app
vanilla JS (nessun build step) che parla con il server tramite un **contratto HTTP piccolo e ben definito**
(metodi WebDAV + una dozzina di query string tipo `?zip`, `?edit`, `?tokengen`). La strategia proposta è:

1. **Vendorizzare gli asset di superdufs byte-identici** (`index.html`, `index.js`, `index.css`, `favicon.ico`) e
   incorporarli nel binario con `go:embed`. Così si potrà aggiornare anche il frontend da superdufs con uno script.
2. **Implementare in Go, lato rclone, lo stesso contratto HTTP di superdufs** sopra la VFS di rclone. Il JS non
   sa di parlare con rclone.
3. **Tutto il codice nuovo vive in file e cartelle nuove.** Nei file upstream si aggiungono solo **3 righe** in
   `cmd/serve/webdav/webdav.go`, nessuna riga esistente modificata, nessuna dipendenza nuova in `go.mod`.
4. **Tutte le opzioni e i flag esistenti restano invariati e rispettati** (tabella al §6). Si aggiungono solo flag
   nuovi con prefisso `--webui-*`. Con la UI disattivata il comportamento è identico a upstream.

Stima: **circa 12–15 giorni-uomo** per la parità funzionale completa, in 6 fasi (§13).

Rispetto all'integrazione già presente in superdufs (`--rclone-remote`, che lancia `rclone mount` via FUSE e lo
supervisiona), questo approccio elimina FUSE, il supervisore dei processi e il doppio strato di cache: superdufs
legge un mount, il fork legge direttamente la VFS dentro lo stesso processo.

---

## 2. Stato attuale

### 2.1 `rclone serve webdav` oggi

File chiave: `cmd/serve/webdav/webdav.go` (757 righe).

- `newWebDAV()` crea un `libhttp.Server` (router chi) con `WithConfig`, `WithAuth`, `WithTemplate`, poi aggiunge
  i middleware `provider.HoldVFS`, gzip, `Accept-Ranges` e `Server`, e monta `router.Handle("/*", w)` più i
  metodi WebDAV (`COPY`, `LOCK`, `MKCOL`, `MOVE`, `PROPFIND`, `PROPPATCH`, `UNLOCK`).
- `ServeHTTP()`:
  - controlla il path con `checkPath()` (rifiuta `.` e `..`, anche nell'header `Destination`)
  - `GET`/`HEAD` su un path **con slash finale** → `serveDir()`: listing HTML via template Go
    (`lib/http/templates/index.html`, sostituibile con `--template`) e zip con `?download=zip`
  - tutto il resto → `golang.org/x/net/webdav.Handler`, che fa GET dei file con `http.ServeContent` (Range ok),
    `PUT`, `DELETE`, `MKCOL`, `MOVE`, `COPY`, `PROPFIND` e lock in memoria
- L'auth (`--user/--pass`, `--htpasswd`, `--auth-proxy`, `--user-from-header`, certificato client) è un middleware
  chi installato da `libhttp.NewServer` → `initAuth()` su **tutto** il router: o è richiesta ovunque, o da nessuna parte.

Comportamenti di `x/net/webdav` rilevanti, verificati nel sorgente v0.61.0:

| Richiesta | Risposta |
|---|---|
| `PUT` con cartella padre inesistente | **409 Conflict** (non crea le cartelle padre) |
| `GET`/`HEAD` su cartella **senza** slash finale | **405 Method Not Allowed** |
| `PATCH` | non gestito (405) |
| `MOVE` con `Overwrite: T` e destinazione esistente | **`RemoveAll(dst)`**, poi `Rename` |

rclone forza `Overwrite: T` quando l'header manca (workaround per golang/go#66059).

### 2.2 Frontend superdufs

- `assets/index.html` (263 righe) con due placeholder: `__ASSETS_PREFIX__` e `__INDEX_DATA__`, che contiene il
  **JSON dei dati della pagina codificato in base64**.
- `assets/index.js` (3399 righe, vanilla, nessuna dipendenza): listing, multi-selezione, upload file e cartelle
  (anche drag & drop), coda di upload con concorrenza configurabile e retry, editor, viewer, resilient player
  (riconnessione, watchdog sul buffer, boost audio, playlist, ripresa della riproduzione), link pubblici con
  bottom-sheet mobile, comandi curl/wget, statistiche disco.
- `assets/index.css` (1825 righe): layout responsive desktop/mobile. Si ottiene gratis vendorizzando il file.

---

## 3. Contratto HTTP usato dal frontend

Ricavato da `index.js` e `server.rs`. È ciò che il fork deve implementare.

### 3.1 Endpoint

| Richiesta del JS | Scopo | Risposta attesa |
|---|---|---|
| `GET <dir>/` | pagina principale | `index.html` con `__INDEX_DATA__` = base64(JSON `IndexData`) e `__ASSETS_PREFIX__` |
| `GET <dir>/?json` | listing (playlist ricorsiva) | JSON `IndexData` |
| `GET <dir>/?sort=<campo>&order=<asc\|desc>` | ordinamento lato server | pagina con `paths` ordinati |
| `GET <dir>/?q=<testo>` | ricerca ricorsiva | pagina con `paths` filtrati |
| `GET <dir>/?zip` | download della cartella | stream zip |
| `HEAD`/`GET <dir>/?zip=multi&paths=<JSON array>` | zip della selezione (max 200 nomi, 8 KB di payload) | HEAD per validare, poi stream zip |
| `GET <file>` | download / stream | contenuto con supporto Range |
| `GET <file>?edit` / `?view` (`&playlist`) | editor / viewer / player | `index.html` con `EditData` (kind `Edit`/`View`) |
| `GET <path>?tokengen` | link di download firmato | token in testo semplice, poi usato come `?token=<t>` |
| `GET ?disk-stats` | spazio disco | `{disk_usage, disk_total, disk_available}` |
| `GET <file>?resume-get` / `?resume-save=<pos>&duration=<d>` / `?resume-clear` | ripresa della riproduzione | JSON della posizione / 200 |
| `CHECKAUTH <dir>[?login]` | verifica del login | nome utente in testo, oppure 401 |
| `LOGOUT <dir>` | logout dalla Basic auth | 401 (il browser svuota le credenziali) |
| `PUT <file>` (body) | upload, nuovo file, salvataggio dall'editor | 2xx; **crea le cartelle padre** (upload di cartelle) |
| `PATCH <file>` + `X-Update-Range: append` | ripresa di un upload interrotto | 2xx |
| `HEAD <file>` | esistenza / offset per la ripresa | 200 + `Content-Length` |
| `DELETE`, `MKCOL`, `MOVE` (`Destination`) | gestione file | 2xx |
| `GET /@public/<token>/<path>[?view\|?zip\|?resume-*]` | link pubblico, **senza auth** | file, cartella navigabile o player |
| `GET __ASSETS_PREFIX__index.js\|index.css\|favicon.ico` | asset | file statici |

`server.rs` gestisce in tutto queste query: `disk-stats edit hash json login noscript paths q resume-clear
resume-get resume-save simple sort token tokengen view zip`. Di queste, **`?simple`, `?hash` e `?noscript` non
sono usate da `index.js`**: sono API per curl e script. Non servono per la parità della UI e restano opzionali.
Per il resilient player il server deve offrire solo Range, i due flag di riconnessione e `?resume-*`: non c'è
transcodifica server-side (`VIDEO_ENCODING.md` riguarda la preparazione dei file, ad esempio `+faststart`).

### 3.2 Strutture dati

**`IndexData`** (pagina cartella e `?json`): `href`, `kind:"Index"`, `uri_prefix`, `allow_upload`, `allow_delete`,
`allow_search`, `allow_archive`, `dir_exists`, `auth`, `user`, `paths[]`, `playback_resume_enabled`,
`publink_enabled`, `publink_is_viewer`, `enable_data_stats`, `show_concurrent_uploads_settings`, `disk_usage`,
`disk_total`, `disk_available`.

**`PathItem`**: `path_type` (`Dir|SymlinkDir|File|SymlinkFile`), `name`, **`mtime` in millisecondi**, `size` (per le
cartelle è il **numero di figli**), `resume_position?`, `publink_token?`.

**`EditData`** (pagine `?edit`/`?view`, struttura diversa): `href`, `kind`, `uri_prefix`, `allow_upload`,
`allow_delete`, `auth`, `user`, `editable` (file ≤ limite e testo, rilevato sul primo KB),
`resilient_player_max_retry`, `resilient_player_reconnect_interval`.

**Vincolo:** `uri_prefix` deve finire con `/`. Il codice di MOVE fa `uri_prefix.slice(0, -1)`. In rclone vale
`BaseURL + "/"`.

**Token di link pubblico:** HMAC-SHA256(secret, `uri_prefix_senza_slash + "/" + path`) troncato a 128 bit, in hex.
Un token di cartella vale per tutto il sottoalbero. Il token viene emesso dal server in ogni `PathItem`.

---

## 4. Gap analysis: funzionalità richieste e situazione in rclone

| Funzionalità | rclone oggi | Cosa va aggiunto nel fork | Note VFS / remoti |
|---|---|---|---|
| Interfaccia superdufs (desktop/mobile) | template HTML statico | rendering di `index.html` con `IndexData`, asset embedded | CSS responsive già incluso negli asset |
| Upload file | `PUT` ok | — | con `--vfs-cache-mode off` l'upload è sequenziale (va bene per PUT) |
| Upload cartelle | `PUT` → 409 se manca il padre | `VFS.MkdirAll(parent)` prima di `PUT`, **solo per richieste dal browser** | — |
| Ripresa upload | assente | `PATCH` + `X-Update-Range: append`; `PUT` dal browser gestito dalla webui (§10.2) | **richiede `--vfs-cache-mode writes` o `full`**: senza cache non si può appendere |
| Download file | ok (Range) | — | — |
| Download cartella | `?download=zip` | alias `?zip` (riusa `vfs.CreateZip`) | traffico pari all'intera cartella |
| Zip della selezione multipla | assente | `?zip=multi&paths=…` con `archive/zip` sulla VFS | `checkPath` su **ogni** elemento di `paths` |
| Modifica file | assente | pagina `?edit` (EditData) + `PUT` | lettura del primo KB per capire se è testo |
| Eliminazione file/cartelle | `DELETE` ok | — | rispetta `--read-only` |
| Spostamento / rinomina | `MOVE` ok | **protezione sovrascrittura** (§9) | `DirMove` lato server se il backend lo supporta |
| Nuova cartella / nuovo file | `MKCOL` / `PUT` ok | — | — |
| Selezione multipla | — | nulla (lato client) + zip multi | — |
| Link diretti di download | assente | `?tokengen` + validazione `?token=` (HMAC, scadenza) | non disponibili con `--auth-proxy` |
| Link pubblici (file e cartelle) | assente | `/@public/<token>/…` **senza auth**, viewer di cartella | non disponibili con `--auth-proxy` |
| Resilient player audio/video | GET con Range ok | pagina `?view`, flag del player | `--vfs-cache-mode full` consigliato per il seek |
| Ripresa della riproduzione | assente | `?resume-*` salvati su disco locale | posizioni in `--webui-resume-dir` |
| Playlist | — | serve solo `?json` | — |
| Ricerca | assente | `?q=` con walk VFS limitato | **costosa sui remoti**: limiti di profondità, risultati e tempo |
| Statistiche disco | assente | `?disk-stats` ← `VFS.Statfs()` (feature `About`) | non tutti i backend la supportano → si nasconde |
| Login / logout | Basic auth del browser | `CHECKAUTH`, `LOGOUT` | auth tutto-o-niente (§10) |
| Client senza JS (curl, wget, lynx) | template HTML | fallback al listing upstream | nessun cambiamento per script e scraper |

**Fuori perimetro** (funzioni server di superdufs non legate al frontend): API S3 (rclone ha già `serve s3`),
cleaner, availability watcher, RBAC per percorso (`--http-dav-config`), render-index/SPA. Per l'RBAC l'alternativa
rclone è `--auth-proxy` con un remote per utente. Un RBAC vero si può valutare in una fase successiva.

---

## 5. Architettura proposta

### 5.1 Principi

1. **Solo aggiunte.** Nessuna riga upstream modificata o rimossa. Le uniche righe aggiunte in file upstream sono
   3 e portano il marcatore `// fork:webui`.
2. **Il codice del fork sta in spazi propri:** la cartella `webui/` alla radice e i file `*_webui*.go` dentro
   `cmd/serve/webdav/`. Upstream non creerà mai file con questi nomi.
3. **Nessuna nuova dipendenza.** Solo standard library (`embed`, `crypto/hmac`, `crypto/sha256`, `archive/zip`,
   `encoding/json`, `encoding/base64`, `net/http`). `go.mod` e `go.sum`, tra i file più modificati upstream,
   restano intatti.
4. **Asset superdufs byte-identici**, più un piccolo `overlay.js` del fork per gli adattamenti (§5.6).
5. **Si riusano le funzioni upstream invece di copiarle:** `checkPath`, `provider.Get`/`HoldVFS`,
   `vfs.CreateZip`, `serve.Error`, `libhttp.CtxGetUser`, `libhttp.MiddlewareCORS` e
   `libhttp.MiddlewareResponseHeaders`. Se upstream corregge un bug di sicurezza in `checkPath`, il fork lo eredita
   automaticamente.

### 5.2 Struttura dei file

```
rclone/
├── cmd/serve/webdav/
│   ├── webdav.go                 # UPSTREAM: +3 righe (§5.3)
│   ├── webdav_webui.go           # FORK: glue (flag, middleware, accesso a checkPath/provider)
│   └── webdav_webui_test.go      # FORK: test di contratto e di sicurezza
└── webui/                        # FORK: motore indipendente da webdav
    ├── options.go                # OptionsInfo dei flag --webui-*
    ├── handler.go                # dispatch: dir/file/query → handler
    ├── listing.go                # IndexData/PathItem, sort, ?json, ?q
    ├── pages.go                  # render di index.html (Index/Edit/View), fallback noscript
    ├── archive.go                # ?zip, ?zip=multi
    ├── upload.go                 # PUT con mkdir dei padri, PATCH append
    ├── tokens.go                 # ?tokengen / ?token (HMAC + scadenza)
    ├── publink.go                # /@public/, token di sottoalbero, viewer
    ├── resume.go                 # ?resume-* (JSON su disco locale, retention)
    ├── stats.go                  # ?disk-stats
    ├── assets/                   # VENDOR da superdufs (byte-identici) + SOURCE (commit)
    ├── overlay/overlay.js        # FORK: adattamenti senza toccare index.js
    ├── scripts/sync-assets.sh    # aggiorna gli asset da superdufs
    └── FORK.md                   # regole del fork, procedura di sync
```

Il motore `webui/` non dipende da `cmd/serve/webdav`: in futuro si potrà agganciare anche a `serve http` con la
stessa glue di 3 righe.

### 5.3 Le uniche 3 righe aggiunte a un file upstream

```go
// cmd/serve/webdav/webdav.go

type WebDAV struct {
	// ... campi esistenti invariati ...
	etagHashType  hash.Type
	webui         *webuiGlue // fork:webui
}

	w.server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(w.opt.HTTP),
		libhttp.WithAuth(w.opt.Auth),
		libhttp.WithTemplate(w.opt.Template),
		w.webuiPreAuth(), // fork:webui
	)

	router.Use(
		w.provider.HoldVFS,
		// ... invariato ...
	)
	w.webuiUse(router) // fork:webui
	router.Handle("/*", w)
```

- I flag si registrano senza toccare `webdav.go`. In `webdav_webui.go`, `init()` chiama
  `fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "webdav_webui", …})` e
  `flags.AddFlagsFromOptions(Command.Flags(), "", webui.OptionsInfo)`. Stesso package, file nuovo.
- `w.webuiUse(router)` va messo **prima** di `router.Handle`, altrimenti chi va in panic (`Use` dopo le rotte).
  Un test lo verifica.
- `libhttp.Option` è `type Option func(*Server)` (esportato, `server.go:271`), quindi `webuiPreAuth()` può restituire
  una closure che chiama `s.Router().Use(...)`.

### 5.4 Pipeline di una richiesta

```
richiesta
  │
  ├─[1] webui pre-auth (registrato via Option, quindi PRIMO della catena)
  │     • /@public/<token>/…  → servito qui, senza auth (GET/HEAD soltanto)
  │     • ?token=<t> valido   → servito qui (GET/HEAD soltanto)
  │     • altrimenti          → next
  │     (riapplica StripPrefix(BaseURL), CORS e ResponseHeaders con le funzioni esportate da libhttp)
  │
  ├─ StripPrefix(BaseURL) → CORS → ResponseHeaders → AUTH (upstream, invariato)
  ├─ HoldVFS → gzip → Accept-Ranges → Server                  (upstream, invariato)
  │
  ├─[2] webui middleware (autenticato)
  │     • asset, CHECKAUTH, LOGOUT
  │     • GET dir/ (browser) → pagina UI; ?json ?q ?zip ?zip=multi ?disk-stats ?tokengen
  │     • GET file ?edit ?view ?resume-*
  │     • PUT (browser) → MkdirAll del padre, poi next
  │     • PATCH append (browser)
  │     • MOVE (browser) → protezione sovrascrittura (§9), poi next
  │     • ogni path passa da checkPath() prima di toccare la VFS
  │     • tutto il resto, compreso ?download=zip → next
  │
  └─ WebDAV.ServeHTTP upstream (invariato): checkPath, serveDir classico, x/net/webdav
```

### 5.5 Middleware pre-auth: cosa garantire

Funziona perché `NewServer` applica le `Option` **prima** delle sue chiamate `Use` (`server.go:345`). Questo ordine è
implicito: upstream potrebbe cambiarlo **senza generare conflitti di merge**. Servono quindi test che falliscano
se l'ordine cambia:

- un link pubblico valido risponde 200 **senza credenziali** con `--user/--pass` attivi
- un path privato senza credenziali risponde ancora **401**
- `?token=` scaduto o manomesso → 401/403; token valido su un path diverso → rifiutato
- `/@public/` e `?token=` accettano solo `GET` e `HEAD` (`PUT`, `DELETE`, `MOVE` → 405)
- `checkPath` viene applicato al path pubblico e a ogni elemento di `paths` nello zip multi
- gli header CORS e `--response-header` sono presenti anche sulle risposte pubbliche
- con `--auth-proxy` i link pubblici e i token sono disabilitati: il provider non ha una VFS senza credenziali

### 5.6 Asset: vendoring byte-identico più overlay

- `webui/scripts/sync-assets.sh <path-superdufs> [commit]` copia `assets/*`, scrive `webui/assets/SOURCE` (commit e
  sha256 di ciascun file) e lancia i test di contratto.
- **`overlay.js`** viene caricato dopo `index.js`, iniettato dal server in fase di render senza modificare
  `index.html`. Le funzioni di `index.js` sono dichiarazioni globali di script classico (verificato:
  `function formatDirSizeParts`, riga 2640), quindi l'overlay può ridefinirle. L'avvio di `index.js` avviene in
  `DOMContentLoaded` (riga 163), cioè dopo l'esecuzione di entrambi gli script: l'override è attivo prima del primo
  render. Usi previsti:
  - `formatDirSizeParts(size)`: se `size < 0` mostra "—" invece di "N items" (vedi §10.1)
  - etichette o brand "rclone"
  - eventuali workaround futuri, senza fork del JS
- Gli asset sono serviti da `__ASSETS_PREFIX__` = `<baseurl>/__webui_v<ver>__/` con
  `Cache-Control: immutable` e ETag. Il prefisso cambia a ogni versione, quindi non servono cache-busting manuali.

---

## 6. Interazione con le opzioni e i flag esistenti

Nessun flag esistente cambia nome, default o significato. La UI legge i flag esistenti e si adegua.

| Flag / funzione esistente | Comportamento con la web UI attiva |
|---|---|
| `--read-only` (VFS) | `allow_upload = allow_delete = false`: la UI nasconde upload, eliminazione, spostamento e creazione. La VFS rifiuta comunque (EROFS). Letto per singola VFS, quindi vale anche con `--auth-proxy` |
| `--disable-dir-list` | nessuna pagina UI e nessun listing (`?json`, `?q`). Restano solo le operazioni WebDAV. Di fatto la UI è spenta |
| `--disable-zip` | `allow_archive = false`; `?zip` e `?zip=multi` → 404. `?download=zip` resta disabilitato come in upstream |
| `--template` | **se impostato esplicitamente vince il template utente**: listing classico e UI spenta, con un messaggio a log. Così anche i test golden upstream (che impostano `Template.Path`) passano invariati |
| `--baseurl` | `uri_prefix = baseurl + "/"`; asset e `/@public/` sotto la baseurl |
| `--allow-origin`, `--response-header` | invariati; riapplicati anche dal middleware pre-auth |
| `--user`/`--pass`/`--htpasswd`/`--realm`/`--salt` | auth invariata; `auth=true`, `user` dal contesto (`CtxGetUser`) |
| `--user-from-header`, certificato client (`--client-ca`) | come sopra: l'auth la fa il middleware upstream |
| `--auth-proxy` | funziona tutto tranne link pubblici e `?tokengen` (`publink_enabled=false`) |
| `--etag-hash` | invariato per WebDAV; gli asset UI hanno ETag propri |
| `--no-modtime` | `mtime = 0` nel listing |
| `--vfs-cache-mode` | `off`: niente ripresa degli upload (PATCH → 405); consigliato `writes`, oppure `full` per il player |
| filtri `--include/--exclude/--filter*` | applicati dalla VFS: listing, ricerca e zip li rispettano |
| `--dir-cache-time`, `--poll-interval` | determinano la freschezza del listing UI, come per WebDAV |
| `-L`, `--local-links` | rclone non espone symlink: `path_type` è sempre `Dir` o `File` |
| `--addr` (anche `unix://`), `--cert`/`--key`, timeout | invariati |
| query `?download=zip` | continua a funzionare (passa al codice upstream) |
| rc `serve/start` (`type=webdav`) | le opzioni webui sono nel blocco globale `webdav_webui` (impostabile con `options/set`), non per istanza. Per istanza servirebbe toccare `Options`, che è codice upstream: rinviato |
| gzip (Accept-Encoding) | si applica anche a HTML e JS della UI |

---

## 7. Nuovi flag (solo aggiunte)

Prefisso unico `--webui`. Se upstream introducesse un flag con lo stesso nome, pflag andrebbe in panic all'avvio:
lo intercetta subito il test `go test ./cmd/serve/webdav/` nella CI del fork.

| Flag | Default | Equivalente superdufs |
|---|---|---|
| `--webui` | **da decidere (§14)** | — |
| `--webui-search` | `true` | `--allow-search` |
| `--webui-search-max-depth` / `--webui-search-max-results` / `--webui-search-timeout` | `8` / `1000` / `30s` | — (nuovo, necessario sui remoti) |
| `--webui-publink-paths` | vuoto (link pubblici disattivati) | `--publink-enabled` |
| `--webui-publink-secret` | casuale per processo | `--publink-secret` |
| `--webui-token-secret` / `--webui-token-expiry` | casuale / `24h` | (chiave derivata dall'utente in dufs) |
| `--webui-resume` | `false` | `--playback-resume-enabled` |
| `--webui-resume-dir` / `--webui-resume-retention` | `<cache-dir>/webui/resume` / `30d` | `--playback-resume-dir` / `-retention` |
| `--webui-player-max-retry` / `--webui-player-reconnect-interval` | come superdufs | `--resilient-player-*` |
| `--webui-disk-stats` | `false` | `--enable-data-stats` |
| `--webui-upload-concurrency-settings` | `false` | `--show-concurrent-uploads-settings` |
| `--webui-edit-max-size` | come superdufs | `EDITABLE_TEXT_MAX_SIZE` |
| `--webui-dir-count` | `false` | — (conteggio dei figli: costoso, §10.1) |
| `--webui-multi-zip-max` | `200` | costante JS |

Per superdufs già esistono gli equivalenti rclone: `--auth` → `--user/--pass/--htpasswd`; `--allow-upload/delete` →
inverso di `--read-only`; `--allow-archive` → inverso di `--disable-zip`; `--hidden` → filtri `--exclude`;
`--path-prefix` → `--baseurl`; `--tls-cert/key` → `--cert/--key`; `--enable-cors` → `--allow-origin`;
`--bind/--port` → `--addr`.

---

## 8. Cambiamenti visibili a client WebDAV non-browser e come limitarli

Con la UI spenta non cambia nulla. Con la UI attiva:

| # | Cambiamento | Come si limita |
|---|---|---|
| 1 | `GET dir/` restituisce la pagina UI invece del template | solo con `Accept: text/html` e user-agent non CLI. curl, wget, lynx e lo user-agent `rclone/…` (backend `http` di rclone che legge il listing HTML) ricevono **il listing upstream**. Nessun impatto su script e scraper |
| 2 | `PUT` crea le cartelle padre | solo se è presente l'header `Origin`, che i browser inviano su ogni fetch/XHR non-GET/HEAD e i client DAV no. I client DAV continuano a ricevere 409 come da RFC 4918 |
| 3 | `PATCH` + `X-Update-Range` supportato | solo con `Origin`; per gli altri 405 come oggi |
| 4 | `MOVE` su una cartella esistente senza header `Overwrite` → 412 | solo con `Origin` (§9). I client DAV mantengono il default `T` |
| 5 | metodi `CHECKAUTH` e `LOGOUT` | prima 405; innocuo |
| 6 | query `?json ?q ?zip ?edit ?view ?tokengen ?disk-stats ?resume-*` su GET | prima la query veniva ignorata. Impatto trascurabile, va documentato |
| 7 | path riservati `__webui_v…__/` e **`/@public/`** | un file o una cartella del remote con questi nomi nella radice viene oscurato. Il prefisso `@public/` è fisso nel JS (righe 534, 899, 2046): è un limite noto, come in dufs |

`HEAD` su una cartella **non** viene cambiato (resta 405): non si può limitare ai browser, perché la HEAD
same-origin non porta `Origin`. Il problema che causerebbe si risolve lato MOVE (§9).

**Criterio di accettazione:** `cmd/serve/webdav/webdav_test.go` upstream passa **senza modifiche** sia con
`--webui=true` sia con `--webui=false`.

- Tre avvii del server nei test impostano `Template.Path`: `TestWebDav`, `TestHTTPFunction` (golden) e il server
  condiviso alla riga 277. Con la regola "il template esplicito vince" la UI resta spenta, quindi passano in
  entrambe le modalità.
- **Da verificare in F1:** `TestRc`, `TestNewWebDAVError`, `TestAuthProxyDownloadOutlivesCache` e
  `TestWebDavPathTraversal` non impostano il template, quindi girano con la UI attiva. Dovrebbero passare:
  il middleware chiama `checkPath` per primo, e la guard su MOVE scatta solo con `Origin`, quindi
  `TestMoveDefaultsToOverwrite` resta valido.
- Le opzioni webui sono in un blocco globale, non in `Opt`. Per eseguire la suite nelle due modalità,
  `webdav_webui_test.go` imposta un toggle a livello di package e richiama direttamente le funzioni di test upstream
  (`TestWebDav(t)`, `TestHTTPFunction(t)`, …) dentro subtest. Nessun file upstream viene toccato.

---

## 9. Rischio di perdita dati: MOVE su una cartella esistente

Percorso verificato leggendo il codice (`index.js:1010-1033`, `webdav.go:429`, `x/net/webdav/file.go:618`), non
ancora riprodotto end-to-end:

1. L'utente sposta o rinomina la cartella `A` in `B`, e `B` **esiste già**.
2. Il JS fa `HEAD /B` (senza slash) per chiedere conferma della sovrascrittura.
3. `x/net/webdav` risponde **405** su una cartella senza slash. Il JS chiede conferma solo se riceve `200`,
   quindi **nessuna conferma**.
4. Il JS invia `MOVE` senza header `Overwrite`; rclone lo imposta a `T`.
5. `moveFiles` con `overwrite=true` esegue **`RemoveAll(B)`**, poi rinomina `A` in `B`.
   **Il contenuto di B viene cancellato senza preavviso.**

Su dufs non succede, perché la HEAD su una cartella restituisce 200 e il JS chiede conferma.

**Fix nel middleware webui:** per un `MOVE` con `Origin`, senza `Overwrite` e con destinazione che è una
**cartella esistente** → rispondere `412 Precondition Failed` con il messaggio "Destination folder exists". Il JS
mostra l'alert "Cannot move…". Se la destinazione è un **file**, il JS ha già chiesto conferma (HEAD 200), quindi la
sovrascrittura resta permessa. Serve un test e2e dedicato.

---

## 10. Limiti e differenze di parità

### 10.1 Dimensione delle cartelle

superdufs mostra "N items" contando i figli di ogni sottocartella. Su un remote cloud significa **un listing per
ogni sottocartella a ogni visualizzazione** (N+1 chiamate API): lento e costoso. Proposta: `size = -1` per le
cartelle e `overlay.js` mostra "—". Il conteggio vero si attiva con `--webui-dir-count`, limitato a 1000 come in
superdufs. Ha senso su remoti locali o veloci.

### 10.2 Upload interrotti e ripresa

Problema verificato nel sorgente (`x/net/webdav` `handlePut`, righe 287-307): dopo `io.Copy` viene chiamato
**`f.Close()` anche se la copia è fallita**, ad esempio per una connessione caduta a metà upload. La chiusura
normale dell'handle VFS finalizza il file, quindi **un upload interrotto lascia sul remote un file troncato**, con
qualunque cache mode. È così già oggi in upstream, per qualsiasi client WebDAV.

Il retry del JS fa `HEAD` per leggere la dimensione già caricata, poi `PATCH append` dall'offset:

- con `--vfs-cache-mode writes` o `full` l'append funziona e la ripresa è corretta, come in superdufs
- con `off` il `PATCH` risponde 405: l'utente resta con un file troncato e un retry che fallisce

**Proposta:** la webui gestisce da sé i `PUT` dei browser (con `Origin`), con la stessa logica di `handlePut` più
il confronto tra i byte ricevuti e il `Content-Length`. Se l'upload è incompleto:

- **cache `off`**: elimina il parziale. La `HEAD` del retry restituisce 404, quindi il JS riparte da offset 0 e il
  retry funziona, solo senza ripresa
- **cache `writes`/`full`**: tiene il parziale, che serve all'append

I `PUT` dei client DAV continuano a passare dal codice upstream senza modifiche. Conviene comunque un warning
all'avvio quando la UI è attiva con cache `off`.

### 10.3 Ricerca

È un walk ricorsivo sul remote, quindi va limitata con profondità, numero di risultati e timeout (flag §7). Usa la
VFS, che rispetta filtri e dir-cache.

### 10.4 Autenticazione

In rclone l'auth è tutto-o-niente: non esiste "lettura anonima, login per scrivere" come in dufs. Con l'auth
attiva l'utente è sempre loggato, il bottone Login non compare e Logout funziona tramite `LOGOUT` → 401. Senza auth
il JS nasconde entrambi. Un accesso anonimo misto richiederebbe di modificare `initAuth`, cioè codice upstream:
escluso.

### 10.5 `--auth-proxy`

Ogni utente ha la sua VFS, che il provider ottiene solo con le credenziali. I link pubblici e i token di download
non hanno credenziali, quindi sono disabilitati in questa modalità.

### 10.6 Hash

superdufs ha `?hash` (sha256), ma `index.js` non lo usa. Si può implementare in seguito con
`fs.Object.Hash` quando il backend lo supporta.

---

## 11. Sicurezza

- **Path traversal:** ogni path della UI passa da `checkPath()` upstream. Vale anche per i path dei link pubblici,
  gli elementi di `paths` nello zip multi e l'header `Destination`.
- **Token:** HMAC-SHA256 confrontato con `hmac.Equal` (tempo costante). I link pubblici sono legati al path e
  coprono il sottoalbero solo per i token di cartella. I token di download sono legati a utente, path e scadenza.
  I secret sono casuali per processo, salvo i flag espliciti (per avere link stabili tra un riavvio e l'altro).
- **Bypass dell'auth limitato** a `GET`/`HEAD` su `/@public/` e `?token=`, con i test negativi del §5.5.
- **CSRF:** `PUT`, `DELETE`, `MOVE` e `PATCH` cross-origin richiedono un preflight CORS, che fallisce se
  `--allow-origin` non è impostato. `?resume-save` è un GET con effetto collaterale minimo, come in superdufs.
- **XSS:** i nomi file arrivano dal remote. Bisogna verificare che `index.js` faccia escaping in tutti i punti in cui
  usa `innerHTML`; superdufs ha già `tests/security_hardening.rs` come riferimento. In più, la pagina va servita con
  `Content-Security-Policy` restrittiva e `X-Content-Type-Options: nosniff`.
- **XSS memorizzato tramite file caricati:** i file `.html` e `.svg` (che sono in `IFRAME_FORMATS`) vengono serviti
  dalla stessa origin della UI. Un file malevolo aperto nel viewer può eseguire JS con la sessione dell'utente e
  quindi fare `DELETE` e `MOVE`. La CSP della pagina UI non copre le risposte dei file. **Mitigazione:** con la UI
  attiva, aggiungere `Content-Security-Policy: sandbox` alle `GET` dei file, almeno su `/@public/` e per i tipi
  HTML/SVG/XML. Per i client DAV non cambia nulla.
- **Dati di resume:** salvati su disco locale (`0700`), mai sul remote, quindi nessuna esposizione via listing.

---

## 12. Strategia di merge con upstream

### 12.1 Setup git

```bash
cd rclone
git remote rename origin upstream             # https://github.com/rclone/rclone.git
git remote add origin <url-del-fork>          # es. https://github.com/<utente>/rclone
git config rerere.enabled true                # ricorda le risoluzioni dei conflitti
git config rerere.autoupdate true
```

Branch:

- `master`: specchio di `upstream/master`, nessun commit del fork
- `webui/main`: `master` + commit del fork (oggi `feat/webdav-frontend`)
- release: tag `vX.Y.Z-webui.N` creati su merge di tag upstream stabili (`v1.7x.y`)

### 12.2 Procedura di sync (script `webui/scripts/sync-upstream.sh`)

```bash
git fetch upstream --tags
git checkout master && git merge --ff-only upstream/master
git checkout webui/main && git merge --no-ff <tag-upstream>    # oppure upstream/master
go build ./... && go test ./webui/... ./cmd/serve/webdav/... ./lib/http/...
webui/scripts/check-footprint.sh                               # vedi sotto
```

Il **merge** (non il rebase) è consigliato perché il branch è condiviso e pubblicato. Con un'impronta di 3 righe,
anche un rebase periodico della serie di patch sarebbe banale.

### 12.3 Regole che rendono i merge senza conflitti

1. **Footprint check** (`check-footprint.sh`, in CI): `git diff --stat upstream/master...HEAD` deve toccare solo
   `webui/**`, `cmd/serve/webdav/*_webui*.go` e `cmd/serve/webdav/webdav.go`. Per `webdav.go` sono ammesse solo righe
   aggiunte con `// fork:webui`. Qualsiasi altra modifica fa fallire la CI.
2. **Nessuna modifica** a `go.mod`, `go.sum`, `Makefile`, `VERSION`, `docs/`, `MANUAL.*`, `lib/http/**`, `vfs/**`.
3. **Versione del fork via ldflags** nello script di build del fork, non modificando `VERSION`:
   `-X github.com/rclone/rclone/fs.Version=v1.7x.y-webui.N`.
4. **Documentazione del fork** in `webui/FORK.md` e nei `Help:` dei flag, mai nella stringa `Long` del comando né
   nei doc autogenerati.
5. **Test upstream sempre verdi e mai modificati:** sono la prova che il merge non ha rotto nulla.

### 12.4 Rete di sicurezza contro cambiamenti semantici (che non generano conflitti)

Il rischio vero non è il conflitto testuale ma upstream che cambia un comportamento su cui il fork conta. Va
coperto con test, non con il diff:

- **Test di contratto** (`webdav_webui_test.go`, backend `:memory:` o `local` in tmpdir con `httptest`): una
  richiesta per ogni riga della tabella §3.1, con verifica di status, header e JSON. Coprono anche: ordine
  Option/Use (§5.5), 405 su HEAD di una cartella (§9), 409 su PUT DAV senza `Origin`, `?download=zip`.
- **Test "API surface lock":** estrae da `webui/assets/index.js` tutti i metodi HTTP e le query `?xxx` usati, e
  fallisce se ne compare uno non gestito. Così un aggiornamento degli asset di superdufs che introduce un nuovo
  endpoint viene intercettato subito.
- **Test del frontend:** i test `node:test` di superdufs (`tests/frontend/*.test.mjs` con harness DOM stub) si
  possono riusare per verificare che `overlay.js` non rompa `index.js`. Se Node manca, vengono saltati.

### 12.5 Due upstream, due canali indipendenti

| Upstream | Cosa si sincronizza | Strumento | Rischio |
|---|---|---|---|
| rclone | tutto tranne `webui/` | `git merge` di tag | quasi nullo (3 righe) |
| superdufs | solo `webui/assets/` | `sync-assets.sh` + `SOURCE` + API surface lock | nuovi endpoint → il test segnala cosa implementare in Go |

---

## 13. Piano di lavoro

| Fase | Contenuto | Stima | Criterio di uscita |
|---|---|---|---|
| **F0** Setup | remote, branch, `rerere`, script di sync e footprint, CI (build, test upstream, footprint) | 0,5 g | CI verde su upstream puro |
| **F1** Scheletro | `webui/options.go`, glue di 3 righe, embed degli asset, overlay, pagina Index, `?json`, sort, fallback noscript e template | 2 g | `rclone serve webdav :memory:` mostra la UI; test upstream invariati verdi in entrambe le modalità |
| **F2** Operazioni sui file | PUT con mkdir dei padri (`Origin`), PATCH append, guard su MOVE (§9), CHECKAUTH/LOGOUT, pagine `?edit` e `?view` | 2,5 g | upload di cartelle e ripresa (con cache `writes`), editor, spostamento senza perdita dati: test e2e |
| **F3** Archivi, ricerca, statistiche | `?zip`, `?zip=multi`, `?q` con limiti, `?disk-stats` | 2 g | zip validi (anche cartelle vuote e nomi unicode), ricerca limitata |
| **F4** Token e link pubblici | middleware pre-auth, `?tokengen`/`?token`, `/@public/` per file e cartelle, viewer | 3 g | tutta la suite negativa del §5.5 verde |
| **F5** Player e resume | `?resume-*`, `resume_position` nel listing, flag del player, retention | 1,5 g | ripresa della riproduzione anche via link pubblico |
| **F6** Hardening e rilascio | CSP e header, API surface lock, documentazione `FORK.md`, build multi-arch, test su remoti reali (S3, SFTP, Drive) | 2,5 g | checklist di sicurezza ok, release `-webui.1` |
| | **Totale** | **~14 g** | |

---

## 14. Decisioni aperte

1. **UI attiva di default?** Fa cambiare il comportamento predefinito di `serve webdav`, cosa che va ponderata
   rispetto a "aggiungere, non modificare".
   - **(a) Raccomandata:** `--webui=true` di default nel fork. Il comando `rclone serve webdav --addr 0.0.0.0:8000 remote:`
     mostra subito superdufs, come richiesto. Il comportamento upstream si ottiene con `--webui=false` (oppure con
     `--template` esplicito). I golden test upstream passano comunque (§8).
   - **(b)** `--webui=false` di default: il comportamento resta identico a upstream, ma la UI va attivata a ogni
     avvio con `--webui`.
2. **Nome dei flag:** `--webui-*` (proposto) oppure un prefisso più distintivo come `--superui-*`, che riduce a zero il
   rischio di collisione con flag upstream futuri.
3. **Posizione del motore:** `webui/` alla radice (proposto: confine del fork evidente) oppure
   `cmd/serve/webdav/webui/`.
4. **RBAC per percorso** (`--http-dav-config` di superdufs): escluso dalla v1. Va confermato.
5. **Conteggio dei figli delle cartelle:** default "—" (proposto) oppure conteggio attivo.

---

## 15. Alternative valutate e scartate

| Alternativa | Perché no |
|---|---|
| Solo `--template` con un template personalizzato (zero fork) | il template riceve solo `Entries`/`Breadcrumb`: niente `?json`, `?edit`/`?view`, zip multi, PATCH, token, link pubblici, resume. Copre forse il 30% della UI |
| Modificare direttamente `webdav.go`/`serveDir` | conflitti a ogni merge: il file ha avuto 21 commit negli ultimi 2 anni, tra cui fix di sicurezza recenti (path traversal, set 2026) |
| Modificare `lib/http` (`initAuth`) per il bypass dell'auth | 15 commit in 2 anni; il middleware pre-auth via `Option` ottiene lo stesso risultato senza toccarlo |
| Riscrivere il frontend in un framework moderno | perde la sincronizzazione con superdufs e moltiplica il lavoro |
| superdufs davanti a `rclone mount` (oggi: `--rclone-remote`) | esiste già, ma dipende da FUSE, da un supervisore dei processi e da una doppia cache. Il fork serve la VFS direttamente |
| Reverse proxy (superdufs → `rclone serve webdav`) | due processi, la UI non sa usare la VFS (resume, statistiche, link) e servirebbe un adattatore |
