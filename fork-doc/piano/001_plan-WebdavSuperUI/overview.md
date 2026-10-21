# WebdavSuperUI — Plan overview

Redatto il 2026-10-21 da Opus 5.5 (supervisore). Cartella: `fork-doc/piano/001_plan-WebdavSuperUI/` (fino a T-A004 era `docs/plans/001_plan-WebdavSuperUI/`).
L'esecuzione parte da [STATE.md](STATE.md): readiness e progresso live sono solo lì.
Analisi di origine (fuori repo): `ANALISI_WEBUI.md`.

## Goal and reference scenario

Fork di rclone in cui `rclone serve webdav` espone il frontend di superdufs (asset vendorizzati
byte-identici) con tutte le sue funzionalità, implementando in Go il contratto HTTP che il JS usa,
sopra la VFS di rclone. Solo aggiunte: nessuna opzione/flag esistente cambia; merge da upstream
senza conflitti.

Reference scenario (RS):
```
rclone serve webdav --addr 127.0.0.1:8000 /tmp/rs-root
```
- `GET /` con `Accept: text/html` → pagina superdufs (HTML con `__INDEX_DATA__` base64 di IndexData, asset sotto `/__webui_<hash>__/`).
- `GET /?json` → JSON IndexData con `paths` ordinati (cartelle prima, ordinamento naturale).
- upload di file e cartelle (PUT dal browser con `Origin`, cartelle padre create), download file (Range), download cartella `?zip`, zip selezione `?zip=multi&paths=[...]`.
- `?edit`/`?view` su file → pagina EditData; salvataggio con PUT.
- DELETE, MKCOL, MOVE (con guard sovrascrittura cartelle), selezione multipla (client).
- `?tokengen` + `?token=` (con auth attiva), link pubblici `/@public/<token>/<path>` (con `--webui-publink-paths`).
- player resiliente `?view` + `?resume-get|save|clear` (con `--webui-resume`).
- `curl` (UA `curl/`) su `GET /` → listing upstream invariato.
- Con `--webui=false` o `--template X`: comportamento identico a upstream.

Esclusioni: API S3, cleaner, availability watcher, RBAC per percorso (`--http-dav-config`), render-index/SPA, `?simple`, `?hash`, `?noscript` (API non usate da `index.js`), accesso anonimo misto, modifica di file upstream oltre le 3 righe di glue, nuove dipendenze Go.

## Decisions

| ID | Decision and consequence | Authority/source | Supersedes |
|----|--------------------------|------------------|------------|
| D1 | UI attiva di default (`--webui=true`); `--webui=false` ripristina upstream. | Requisito utente (RS del messaggio iniziale: al lancio del comando deve comparire superdufs) | — |
| D2 | Nessun file upstream modificato salvo 3 righe aggiunte in `cmd/serve/webdav/webdav.go` marcate `// fork:webui`; nessuna riga upstream modificata/rimossa; nessuna modifica a `go.mod`/`go.sum`. | Requisito utente ("aggiungere, non modificare"; merge senza problemi) | — |
| D3 | Motore in package nuovo `github.com/rclone/rclone/webui` (dir `webui/`); glue in `cmd/serve/webdav/webdav_webui.go` (stesso package `webdav`, accede a `checkPath`, `provider`, `opt`). | Supervisore | — |
| D4 | Asset superdufs vendorizzati byte-identici in `webui/assets/` + `webui/assets/SOURCE`; adattamenti solo via `webui/overlay/overlay.js` iniettato dopo `index.js`. | Supervisore (sync con superdufs) | — |
| D5 | Flag nuovi con prefisso `--webui`, registrati come blocco globale `webdav_webui` via `fs.RegisterGlobalOptions` e aggiunti a `Command.Flags()` dal glue. | Supervisore | — |
| D6 | UI attiva solo se `webui.Opt.Enabled && opt.Template.Path == "" && !opt.DisableDirList`; altrimenti nessun middleware webui (comportamento upstream puro). | Supervisore (rispetto flag esistenti) | — |
| D7 | Due middleware: `PreAuth` (registrato via `libhttp.Option` → primo della catena, prima di auth) per asset, `/@public/`, `?token=`; `Handler` (registrato con `router.Use` dopo i middleware upstream, prima di `router.Handle`) per tutto il resto. Ciò che non è UI passa a `next`. | Supervisore (R2) | — |
| D8 | Cambiamenti visibili ai client non-browser limitati: PUT/PATCH/MOVE webui solo con header `Origin`; pagina UI solo con `Accept: text/html` e UA non-CLI; HEAD su cartelle invariato. | Supervisore (R1, R3) | — |
| D9 | MOVE dal browser (con `Origin`, senza `Overwrite`) verso cartella esistente → 412 "Destination folder exists". | Supervisore (R3, rischio perdita dati) | — |
| D10 | PUT dal browser gestito dalla webui: MkdirAll del padre, confronto byte/Content-Length; upload incompleto con cache < writes → parziale rimosso; con writes/full → tenuto per PATCH append. PATCH `X-Update-Range: append` richiede cache ≥ writes, altrimenti 405. | Supervisore (R3, R4) | — |
| D11 | Dimensione cartelle `-1` (overlay mostra "—"); `--webui-dir-count` abilita il conteggio (max 1000). | Supervisore (costo remoti) | — |
| D12 | Token download: `<expMs>.<b64url(user)>.<hex(HMAC(tokKey, user\0remote\0exp)[:16])>`, scadenza `--webui-token-expiry` (72h); servito da PreAuth solo se `UsingAuth && !IsProxy && Authorization assente`. | Supervisore | — |
| D13 | Link pubblici: token = hex(HMAC(pubKey, remote)[:16]); un token di cartella copre il sottoalbero; abilitati solo per percorsi `--webui-publink-paths`; disabilitati con `--auth-proxy`. Chiavi derivate da `--webui-secret` (random per processo se vuoto). | Supervisore | — |
| D14 | Resume su disco locale (`--webui-resume-dir`, default `<cache-dir>/webui/resume`), chiave sha256(`fs.ConfigString(VFS.Fs())`+"\x00"+remote); pulizia lazy (max 1/24h) per retention. | Supervisore | — |
| D15 | Header di sicurezza: pagine UI con `X-Content-Type-Options: nosniff`, `Cache-Control: no-cache`, CSP `frame-ancestors 'self'; object-src 'none'; base-uri 'self'`; GET di file html/svg/xml con UI attiva → `Content-Security-Policy: sandbox`. | Supervisore (XSS memorizzato) | — |
| D16 | Ricerca `?q` con BFS VFS limitata da `--webui-search-max-depth` (8), `--webui-search-max-results` (1000), `--webui-search-timeout` (30s); risultato parziale accettato. | Supervisore | — |
| D17 | Documentazione del fork in `webui/README.md` (italiano); README upstream intoccato. | Supervisore (D2) | — |

## Open questions

Nessuna bloccante. Default D1 derivato dal requisito; reversibile con un solo default in `webui/options.go`.

## Architecture

```
richiesta → [PreAuth webui] → StripPrefix → CORS → RespHeaders → AUTH → HoldVFS → gzip → headers
          → [Handler webui] → WebDAV.ServeHTTP upstream (checkPath, serveDir, x/net/webdav)
```
- `webui.UI` (creato in `webuiPreAuth()` durante `libhttp.NewServer`) contiene `Config` (callback verso WebDAV), `Options`, chiavi HMAC, store resume, cache asset.
- PreAuth: rimuove BaseURL da sé, riapplica `libhttp.MiddlewareCORS` e `MiddlewareResponseHeaders`, avvolge con `HoldVFS`.
- Handler: dispatch per metodo/tipo nodo/query; ogni path passa da `checkPath`; errori di path → `next`.
- VFS: `VFS.Stat`, `Dir.ReadDirAll`, `VFS.OpenFile`, `VFS.MkdirAll`, `VFS.Statfs`, `vfs.CreateZip`.

## Interfaces and compatibility

| Surface | Exact name/type | Default | Errors/conflicts | Compatibility |
|---------|-----------------|---------|------------------|---------------|
| CLI | `--webui` bool | true | — | `false` = upstream |
| CLI | `--webui-search` bool / `--webui-search-max-depth` int / `--webui-search-max-results` int / `--webui-search-timeout` Duration | true / 8 / 1000 / 30s | — | additivo |
| CLI | `--webui-publink-paths` CommaSepList | vuoto (off) | proxy → off | additivo |
| CLI | `--webui-secret` string (Sensitive) | "" → random | — | additivo |
| CLI | `--webui-token-expiry` Duration | 72h | — | additivo |
| CLI | `--webui-resume` bool / `--webui-resume-dir` string / `--webui-resume-retention` Duration | false / "" / 720h | dir non creabile → resume off + log | additivo |
| CLI | `--webui-player-max-retry` int / `--webui-player-reconnect-interval` int (s) | 30 / 10 | — | additivo |
| CLI | `--webui-disk-stats` bool / `--webui-upload-settings` bool | false / false | — | additivo |
| CLI | `--webui-edit-max-size` SizeSuffix | 4Mi | — | additivo |
| CLI | `--webui-dir-count` bool / `--webui-multi-zip-max` int | false / 200 | — | additivo |
| rc | blocco opzioni globali `webdav_webui` | come CLI | — | `options/set` |
| HTTP | contratto §3 di ANALISI_WEBUI.md (dettagli per fase) | — | vedi fasi | solo con UI attiva |

## Phase map

| Logical phase | File | Sub-phase IDs | Depends on | Assignment |
|---------------|------|---------------|------------|------------|
| 0 | [phase_01.md](phase_01.md) | 0.1, 0.2 | none | Sonnet |
| 1 | [phase_02.md](phase_02.md) | 1.1, 1.2, 1.3 | P0 | Sonnet |
| 2 | [phase_03.md](phase_03.md) | 2.1, 2.2 | P1 | Sonnet |
| 3 | [phase_04.md](phase_04.md) | 3.1, 3.2 | P2 | Sonnet |
| 4 | [phase_05.md](phase_05.md) | 4.1, 4.2 | P3 | Sonnet |

No live status in this table.

## Reuse map

| Need | Path and symbol | Contract to preserve |
|------|-----------------|----------------------|
| validazione path | `cmd/serve/webdav/webdav.go — checkPath(name string) error` | rifiuta elementi `.`/`..` |
| VFS per richiesta | `cmd/serve/webdav/webdav.go — (*WebDAV).getVFS(ctx)`; `cmd/serve/proxy/proxy.go — (*Provider).HoldVFS`, `IsProxy()` | errore senza auth in modalità proxy |
| opzioni libhttp | `lib/http/server.go — type Option func(*Server)`, `(*Server).Router()`, `(*Server).UsingAuth()` | Option applicate prima delle `Use` di NewServer (R2) |
| middleware | `lib/http/middleware.go — MiddlewareCORS(string)`, `MiddlewareResponseHeaders([]*fs.HTTPOption)`; `fs.ParseHeaders([]string)` | — |
| utente | `lib/http/context.go — CtxGetUser(ctx)`; fallback `r.BasicAuth()` (proxy non imposta l'utente) | — |
| zip | `vfs/zip.go — CreateZip(ctx, *vfs.Dir, io.Writer)` (Deflate per file, Store per dir) | — |
| errori listing | `lib/http/serve/dir.go — Error(ctx, what, w, text, err)` | — |
| opzioni globali | `fs/registry.go — RegisterGlobalOptions`, `fs.Options`; `fs/config/flags — AddFlagsFromOptions` | default caricati all'init |
| cache dir | `fs/config/config.go — GetCacheDir()` | — |
| identità remote | `fs/newfs.go — ConfigString(f)` | — |

## Research and evidence

Ricerca esterna limitata a comportamenti di protocollo/libreria; verificati su sorgenti locali alle versioni in uso.

| ID | Question / required or optional | Evidence state | Affected decisions / units / tests |
|----|---------------------------------|----------------|------------------------------------|
| R1 | I browser inviano `Origin` su fetch/XHR same-origin non-GET/HEAD? (required) | CONFIRMED | D8, D9, D10; 1.1 |
| R2 | Ordine middleware chi/libhttp: Option prima di `Use`; `Use` dopo rotte → panic (required) | CONFIRMED | D7; 0.1, 3.1 |
| R3 | Comportamento `x/net/webdav` v0.61.0 su PUT/GET dir/PATCH/MOVE (required) | CONFIRMED | D9, D10; 1.1 |
| R4 | VFS: O_APPEND forza handle RW (cache); senza cache non disponibile (required) | CONFIRMED | D10; 1.1 |

### R1 — Origin header
- Sources: WHATWG Fetch Standard, §"Origin header" / "append a request Origin header" (https://fetch.spec.whatwg.org/#origin-header).
- Applicability: browser moderni; richieste con metodo diverso da GET/HEAD includono sempre `Origin` (anche same-origin, mode cors/same-origin).
- Dates: spec living standard; accesso 2026-10-21 (conoscenza consolidata, nessuna fetch live in questa sessione).
- Documented fact: "If request's method is neither GET nor HEAD, then … append (Origin, serializedOrigin)".
- Supervisor inference: i client WebDAV (davfs2, rclone webdav, Windows miniredir, cadaver) non inviano `Origin`.
- Local check: non eseguito; coperto dai test 1.1 (richieste con/ senza `Origin`).
- Decision impact: D8/D9/D10 si attivano solo con `Origin`; i client DAV mantengono il comportamento upstream.
- Unresolved/replacement: none.

### R2 — Ordine middleware
- Sources: `lib/http/server.go` NewServer (opzioni applicate nel loop prima di `MiddlewareStripPrefix`/CORS/headers/`initAuth`); `type Option func(*Server)` (riga ~271); chi v5 `Mux.Use` panics "all middlewares must be defined before routes on a mux".
- Applicability: rclone @ 81e7372f6; chi v5 in go.mod.
- Documented fact: le Option girano prima di ogni `s.mux.Use`; chi esegue i middleware in ordine di registrazione.
- Supervisor inference: un middleware registrato da una Option è il più esterno e vede le richieste prima dell'auth. L'ordine è implicito → test obbligatori (3.1: link pubblico 200 senza credenziali; privato 401).
- Local check: lettura sorgente.
- Decision impact: D7.
- Unresolved/replacement: none.

### R3 — x/net/webdav v0.61.0
- Sources: `$GOMODCACHE/golang.org/x/net@v0.61.0/webdav/webdav.go` handlePut (OpenFile O_RDWR|O_CREATE|O_TRUNC; IsNotExist → 409; `f.Close()` anche dopo errore di copia), handleGetHeadPost (dir → 405), nessun caso PATCH; `file.go` moveFiles (overwrite → `RemoveAll(dst)` poi Rename). `cmd/serve/webdav/webdav.go` ServeHTTP forza `Overwrite: T` se assente.
- Applicability: versione in go.mod.
- Documented fact: come sopra (lettura sorgente).
- Supervisor inference: dalla UI, MOVE su cartella esistente cancella la destinazione senza conferma (HEAD dir → 405 → il JS non chiede conferma). Upload interrotto lascia file troncato.
- Local check: lettura sorgente; riproduzione nei test 1.1.
- Decision impact: D9, D10.
- Unresolved/replacement: none.

### R4 — VFS append
- Sources: `vfs/file.go` (*File).Open: `O_APPEND` → read=true (handle RW); `vfs/read_write.go` offset = size con O_APPEND; `vfscommon.CacheMode` (Off/Minimal/Writes/Full).
- Applicability: rclone @ 81e7372f6.
- Documented fact: handle RW richiede cache; con O_APPEND l'offset parte da size.
- Decision impact: D10 (PATCH solo con cache ≥ writes, altrimenti 405).
- Unresolved/replacement: none.

## Invariants

| ID | Meaning | Guarding test |
|----|---------|---------------|
| I-1 | Con UI disattivata (`--webui=false`, `--template`, `--disable-dir-list`) il comportamento HTTP è identico a upstream. | G-PHASE secondo run (`RCLONE_WEBUI=false`, suite upstream), `TestWebUIDisabledByTemplate`, `TestWebUIDisabledFlag` |
| I-2 | La suite upstream `cmd/serve/webdav` passa invariata anche con UI attiva. | `TestWebUIUpstreamSuiteEnabled` (0.2: `servetest.Run` con UI attiva) + G-PHASE primo run |
| I-3 | Footprint: solo file del fork + 3 righe `// fork:webui` in webdav.go; nessuna riga upstream rimossa/modificata; go.mod/go.sum intatti. | G-FOOT (`webui/scripts/check-footprint.sh`) |
| I-4 | Ogni path toccato dalla webui passa da `checkPath`; path invalidi non vengono mai serviti dalla webui. | `TestWebUIPathTraversal` (1.1), `TestPublicPathTraversal` (3.2) |
| I-5 | Il bypass dell'auth riguarda solo GET/HEAD di asset, `/@public/` validi e `?token=` validi; tutto il resto senza credenziali → 401. | `TestPreAuthBoundaries` (3.1), `TestPublinkBoundaries` (3.2) |
| I-6 | Client senza `Origin` ricevono le risposte upstream per PUT/PATCH/MOVE. | `TestWebUIDavClientUnchanged` (1.1) |
| I-7 | Asset vendorizzati identici a superdufs `SOURCE`. | `TestAssetsMatchSource` (0.1) |

## Risks

| Risk | Design mitigation | Verification |
|------|-------------------|--------------|
| Upstream riordina Option/Use in NewServer senza conflitto | test PreAuth negativi/positivi | 3.1 tests, G-PHASE dopo ogni sync |
| Perdita dati MOVE | D9 | `TestMoveGuardBrowser` |
| File troncati su upload interrotto | D10 | `TestPutIncompleteRemovedNoCache`, `TestPatchAppend` |
| XSS da file caricati | D15 | `TestFileCSPSandbox` |
| Costo listing/ricerca su remoti | D11, D16 | `TestSearchLimits` |
| Nuovo endpoint in superdufs non gestito | API surface lock | `TestAPISurfaceLock` (4.1) |
| Collisione flag con upstream futuro | prefisso `--webui`, panic pflag all'init intercettato | G-PHASE |

## Verification strategy

G-BASE prima del primo edit. Ogni sub-fase: test mirati (`G-U`). Ogni P<N>: G-PHASE (build completa, test webui+webdav+lib/http, lint, footprint). P4: G-FINAL (G-PHASE + race + smoke e2e su binario reale = RS). Comandi in STATE.md §3.

## Model-assignment summary

| Unit(s) | Implementer | Required supervisor review |
|---------|-------------|----------------------------|
| 0.1, 0.2, 1.2, 1.3, 2.1, 2.2, 4.1, 4.2 | Sonnet (agent-2) | Opus a chiusura fase |
| 1.1 | Sonnet (agent-2) | Opus: review focalizzata del diff (integrità dati) prima della chiusura |
| 3.1, 3.2 | Sonnet (agent-2) | Opus: review focalizzata sicurezza (bypass auth, HMAC) prima della chiusura |
| P0–P4 | Opus (agent-1) | — |
| recon | Haiku (agent-3) | non usato: tokensave + supervisore |
