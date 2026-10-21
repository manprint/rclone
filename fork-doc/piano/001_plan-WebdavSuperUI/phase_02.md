# Phase 1 — Operazioni dal browser, editor/viewer, resume

Intent: dalla UI funzionano login/logout, upload (anche in sottocartelle nuove e con ripresa), creazione file,
salvataggio dall'editor, spostamenti sicuri, pagine `?edit`/`?view` con player resiliente e ripresa della
riproduzione; i client WebDAV senza `Origin` vedono le risposte upstream.
Prerequisites: P0 DONE.
Phase closure: P1; review by Opus (agent-1).

## State and ownership contract
Read STATE.md §0–1 first for scope, ownership, recovery, checks, and commit rules.
Open before edits; checkpoint at a recovery boundary; close with evidence. A
delegated worker follows its assigned OPEN unit. Missing design goes to the
configured supervisor.

## Local design context
Plan revision: 1.
- D8: PUT, PATCH e MOVE sono gestiti dalla webui solo se la richiesta ha header `Origin` non vuoto (i browser lo inviano su fetch/XHR non-GET/HEAD — R1); senza `Origin` → `next` (upstream). CHECKAUTH/LOGOUT non esistono in upstream: gestiti sempre.
- D9: MOVE con `Origin`, senza header `Overwrite`, con `Destination` che risolve a una cartella esistente → `412 Precondition Failed`, body `Destination folder exists`. Motivo: la UI fa `HEAD` sulla destinazione senza slash (cartella → 405, nessuna conferma) e poi MOVE senza `Overwrite`; upstream forza `Overwrite: T` → `RemoveAll(dst)` (R3) = perdita dati.
- D10: PUT browser → MkdirAll del padre + scrittura via VFS; PATCH `X-Update-Range: append` → append via VFS (`O_APPEND` richiede cache ≥ writes — R4).
- D15 (parte file): con UI attiva, GET/HEAD di un file con estensione (lowercase) in {`.html`, `.htm`, `.xhtml`, `.svg`, `.svgz`, `.xml`, `.xsl`} → header `Content-Security-Policy: sandbox` impostato prima di passare a `next`.
- D14: store resume su disco locale; chiave file = `hex(sha256(fs.ConfigString(VFS.Fs()) + "\x00" + remote))` + `.json`.
- I-4: ogni path (URL e `Destination`) passa da `cfg.CheckPath`; errore → `next` (upstream risponde 400). Guardia: `TestWebUIPathTraversal`.
- I-6: client senza `Origin` ricevono le stesse risposte upstream. Guardia: `TestWebUIDavClientUnchanged`.
- R3: `x/net/webdav` (v0.61.0): PUT con padre mancante → 409; `handlePut` chiude il file anche dopo errore di copia (file troncato committato); PATCH non supportato; MOVE con overwrite = `RemoveAll(dst)` + `Rename`.
- R4: `VFS.OpenFile` con `O_APPEND` usa un handle RW (cache) con offset = size; senza cache (`CacheModeOff`/`Minimal`) l'append non è affidabile → rifiutato.

### Utilità comuni (in `webui/ui.go` o `webui/util.go`, create in 1.1)
- `remoteFromPath(p string) string` = `strings.Trim(p, "/")`.
- `hasOrigin(r) bool` = `r.Header.Get("Origin") != ""`.
- `(u *UI) user(r) string`: come 0.2 (ctx user, poi `r.BasicAuth()` se `UsingAuth`).
- `(u *UI) destRemote(r) (string, bool)`: `url.Parse(Destination)`; host non vuoto e diverso da `r.Host` → false; `p := u.Path`; prefisso `b := "/" + strings.Trim(cfg.BaseURL(), "/")`; se `b != "/"`: `p` deve avere prefisso `b + "/"` (altrimenti false) e si rimuove `b`; `CheckPath(p)` errore → false; ritorna `remoteFromPath(p)`.
- `httpError(w, status, msg)` = `http.Error`.
- Errori VFS → status: `vfs.ENOENT`/`os.ErrNotExist` → 404; `vfs.EROFS`/`os.ErrPermission`/`vfs.EPERM` → 403; altro → 500 (log `fs.Errorf(remote, "webui: %v", err)`).

### Dispatch nel `Handler` (estende 0.2; l'ordine conta)
1. `CheckPath(r.URL.Path)` errore → next.
2. `CHECKAUTH` → 1.1. `LOGOUT` → 1.1.
3. `PUT` con Origin → 1.1. `PATCH` con Origin e header `X-Update-Range` presente → 1.1. `MOVE` con Origin → guardia 1.1 (poi next).
4. GET/HEAD: query `resume-get`/`resume-save`/`resume-clear` → 1.3 (su qualunque nodo; validazione interna).
5. GET/HEAD su file: `edit` → 1.2 (Edit); `view` → 1.2 (View); altrimenti header D15 + next.
6. GET/HEAD su cartella → 0.2.
7. Altro → next.

## Sub-phases

### 1.1 CHECKAUTH/LOGOUT, PUT e PATCH dal browser, guardia MOVE
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; Opus fa review focalizzata del diff (integrità dati, I-4, I-6) prima della chiusura.
- **Files:**
  READ `webui/ui.go`, `webui/listing.go` (0.2), `vfs/vfs.go` (`OpenFile`, `MkdirAll`, `Remove`, `Stat`, `Opt.CacheMode`, `Opt.ReadOnly`), `vfs/vfscommon/cachemode.go` (`CacheModeWrites`), `vfs/errors.go` (`ENOENT`, `EROFS`, `EPERM`), `cmd/serve/webdav/webdav.go` `ServeHTTP` (Overwrite workaround, Destination check), `cmd/serve/webdav/webdav_test.go` (`TestMoveDefaultsToOverwrite`, helper).
  WRITE `webui/ui.go` (`Handler` dispatch), `cmd/serve/webdav/webdav_webui_test.go`.
  NEW `webui/write.go`, `webui/util.go`.
- **Change:**
  Preconditions: P0 DONE.
  Contract:
  - `CHECKAUTH`: `user := u.user(r)`; se `user == ""` e la query contiene `login` → 401 con `WWW-Authenticate: Basic realm="<realm>"` (`realm = cfg.Realm`, se vuoto `"rclone"`); altrimenti 200, `Content-Type: text/plain; charset=utf-8`, body = user. (Con auth attiva le richieste non autenticate non arrivano qui: l'auth upstream risponde 401.)
  - `LOGOUT`: sempre 401 con lo stesso `WWW-Authenticate`, body vuoto.
  - `PUT` (Origin): `remote` vuoto → next. `VFS.Opt.ReadOnly` → next. `Stat(remote)` esiste ed è cartella → next. `parent := path.Dir(remote)`; se `parent != "."` → `VFS.MkdirAll(parent, 0777)` (errore → status mappato). `h, err := VFS.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)` (errore → mappato). `n, cerr := io.Copy(h, r.Body)`; `clerr := h.Close()`. Incompleto se `cerr != nil || clerr != nil || (r.ContentLength >= 0 && n != r.ContentLength)`. Se incompleto: se `VFS.Opt.CacheMode < vfscommon.CacheModeWrites` → `VFS.Remove(remote)` (ignora ENOENT; log altri errori); risposta 400 `Upload incomplete`. Completo → 201, body vuoto.
  - `PATCH` (Origin + `X-Update-Range`): valore diverso da `append` → 400 `Invalid X-Update-Range Header`. ReadOnly → 403. `node, err := Stat(remote)`: errore → 404; cartella → 405. `VFS.Opt.CacheMode < CacheModeWrites` → 405 body `Append requires --vfs-cache-mode writes or full`. `h := VFS.OpenFile(remote, os.O_WRONLY|os.O_APPEND, 0666)`; copia come PUT; incompleto → 400 `Upload incomplete` (nessuna rimozione: il cache conserva i byte ricevuti); completo → 204.
  - `MOVE` (Origin): se header `Overwrite` presente → next. `dst, ok := u.destRemote(r)`; `!ok` → next. `n, err := VFS.Stat(dst)`; se `err == nil && n.IsDir()` → 412 `Destination folder exists`; altrimenti next.
  Scope: nessun cambiamento per richieste senza Origin; nessun cambiamento a DELETE/MKCOL/COPY/PROPFIND (upstream già corretti per la UI).
  Steps:
  1. S1 — `util.go` (utilità comuni), `write.go` (`serveCheckAuth`, `serveLogout`, `servePut`, `servePatch`, `guardMove`), dispatch passi 1–3 in `Handler`; expected build ok.
  2. S2 — test e2e (sotto) con server reale; expected G-U11 verde; G-BASE verde.
  Recovery boundary: none.
  Failure handling: comportamento upstream diverso da R3 nei test (es. PUT padre mancante ≠ 409) → registra output e chiedi a Opus prima di cambiare l'oracolo.
- **Unit tests:** coperti dagli e2e (logica dipendente dalla VFS); `TestDestRemote` in `webui/util_test.go` (tabella: host diverso → false; baseurl `/pfx` con `/pfx/a/b` → `a/b`; `/other/a` → false; `/a/../b` → false; URL non parsabile → false). Gate G-U11.
- **e2e tests:** in `cmd/serve/webdav/webdav_webui_test.go` (server via helper 0.1; per la cache usa `config.SetCacheDir(t.TempDir())` e `vfsOpt.CacheMode = vfscommon.CacheModeWrites`), gate G-U11:
  - `TestWebUICheckAuthLogout`: senza auth: CHECKAUTH → 200 body ""; CHECKAUTH `?login` → 401 + `WWW-Authenticate` contenente `realm="rclone"`; LOGOUT → 401. Con basic auth: CHECKAUTH con credenziali → 200 body `user`.
  - `TestWebUIPutCreatesParents`: PUT `/new/sub/f.txt` con Origin, body `hello` → 201; file su disco con contenuto `hello`. Stessa PUT senza Origin su `/other/sub/f.txt` → 409 (I-6).
  - `TestPutIncompleteRemovedNoCache`: cache off; richiesta PUT con Origin e `Content-Length: 10` ma body di 3 byte (usa `net.Dial` raw HTTP/1.1 e chiudi la connessione, oppure `http.Request` con `ContentLength=10` e body `io.LimitReader` che termina con errore) → il file non esiste dopo la richiesta. Il client vede l'errore prima che l'handler finisca la `Remove`: verifica con polling (`require.Eventually`, fino a 2s, passo 20ms), non con un'asserzione singola.
  - `TestPatchAppend`: cache writes; PUT `a.bin` (Origin) `abc` → 201; PATCH (Origin, `X-Update-Range: append`) body `def` → 204; GET `a.bin` → `abcdef`. Cache off: PATCH → 405. `X-Update-Range: bytes=0-1` → 400. PATCH su file inesistente → 404.
  - `TestMoveGuardBrowser`: crea `src.txt` e cartella `dst/` con file `keep.txt`; MOVE `/src.txt` con Origin, `Destination: http://host/dst` → 412, `dst/keep.txt` esiste ancora. MOVE con Origin verso `/new.txt` → 201/204 (upstream). MOVE con baseurl `/pfx` e Destination `/pfx/dst` → 412.
  - `TestWebUIDavClientUnchanged`: due server (UI attiva e `webui.Opt.Enabled=false`) sullo stesso tipo di fixture; sequenza senza Origin: PUT padre mancante, PATCH append, MOVE su cartella esistente, CHECKAUTH esclusa; gli status code coincidono tra i due server.
  - `TestWebUIPathTraversal`: con Origin, PUT su `/a/%2e%2e/%2e%2e/x.txt` e MOVE con `Destination: /%2e%2e/x` → status non 2xx, nessun file creato fuori dalla root (controlla la directory padre del TempDir).
- **Done:** contratti sopra soddisfatti, G-U11 verde, G-BASE verde, review Opus registrata in STATE §7, unit chiusa, commit `sub-phase(1.1)`.

### 1.2 Pagine `?edit` / `?view` e CSP sandbox sui file attivi
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P1.
- **Files:**
  READ `webui/listing.go` (render template, header D15 pagine), `webui/data.go`, superdufs `src/server.rs` `handle_edit_file` (riferimento), `vfs/file.go` (`Open`, `Size`).
  WRITE `webui/ui.go` (dispatch passo 5), `webui/data.go` (EditData), `cmd/serve/webdav/webdav_webui_test.go`.
  NEW `webui/edit.go`.
- **Change:**
  Preconditions: 1.1 DONE.
  Contract:
  ```go
  type EditData struct {
      Href                             string  `json:"href"`        // "/" + remote
      Kind                             string  `json:"kind"`        // "Edit" | "View"
      URIPrefix                        string  `json:"uri_prefix"`
      AllowUpload                      bool    `json:"allow_upload"`
      AllowDelete                      bool    `json:"allow_delete"`
      Auth                             bool    `json:"auth"`
      User                             *string `json:"user"`
      Editable                         bool    `json:"editable"`
      ResilientPlayerMaxRetry          int     `json:"resilient_player_max_retry"`          // Opt.PlayerMaxRetry
      ResilientPlayerReconnectInterval int     `json:"resilient_player_reconnect_interval"` // Opt.PlayerReconnectInterval
  }
  ```
  `editable = size <= int64(Opt.EditMaxSize) && isText(prefix)` dove `prefix` = primi 1024 byte letti con `file.Open(os.O_RDONLY)` (errore di lettura → `editable=false`, pagina servita comunque); `isText(b)`: nessun byte 0x00 e `utf8.Valid(b)` dopo aver rimosso al massimo 3 byte finali di una sequenza UTF-8 troncata (se `len(b)==1024`). Render come la pagina Index (stesso template, header D15 pagine). HEAD → solo header. `?view&playlist` = `?view` (il server ignora `playlist`).
  CSP: per GET/HEAD su file senza `edit`/`view` (e senza query resume) con estensione nel set D15 → `w.Header().Set("Content-Security-Policy", "sandbox")` poi next.
  Scope: nessun cambiamento ai file con estensioni diverse; nessun cambiamento con UI disattivata.
  Steps:
  1. S1 — `edit.go` + dispatch; expected build ok.
  2. S2 — test; expected G-U12 verde.
  Recovery boundary: none.
  Failure handling: none noto.
- **Unit tests:** `TestIsText` in `webui/edit_test.go`: ASCII → true; `"a\x00b"` → false; 1024 byte che terminano a metà di `"è"` → true; byte `0xff` → false. Gate G-U12.
- **e2e tests:** gate G-U12:
  - `TestWebUIEditPage`: file `note.txt` = `ciao`; GET `/note.txt?edit` → 200 html; EditData `kind=="Edit"`, `href=="/note.txt"`, `editable==true`, `resilient_player_max_retry==30`, `resilient_player_reconnect_interval==10`. File binario (`\x00\x01`) → `editable==false`. `EditMaxSize=2` → `editable==false`. GET `/v.mp4?view` → `kind=="View"`.
  - `TestFileCSPSandbox`: GET `/x.html` → header `Content-Security-Policy: sandbox` e body del file; GET `/x.txt` → nessun CSP; UI disattivata → GET `/x.html` senza CSP.
- **Done:** G-U12 verde, unit chiusa, commit `sub-phase(1.2)`.

### 1.3 Ripresa riproduzione (store + endpoint + listing)
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P1.
- **Files:**
  READ superdufs `src/playback_resume.rs` (riferimento formato), `webui/listing.go`, `webui/edit.go`, `fs/config/config.go` (`GetCacheDir`), `fs/newfs.go` (`ConfigString`).
  WRITE `webui/ui.go` (dispatch passo 4, creazione store in `New`), `webui/listing.go` (`resume_position`, `playback_resume_enabled`), `webui/README.md` (sezione "Ripresa riproduzione"), test.
  NEW `webui/resume.go`, `webui/resume_test.go`.
- **Change:**
  Preconditions: 1.2 DONE.
  Contract:
  - Store attivo sse `Opt.Resume`. Dir = `Opt.ResumeDir` oppure `filepath.Join(config.GetCacheDir(), "webui", "resume")`; `os.MkdirAll(dir, 0700)` in `New`; errore → log `fs.Errorf(nil, "webui: playback resume disabled: %v", err)` e store disattivato (la UI continua).
  - Record JSON `{"position": float64, "duration": float64|null, "timestamp": int64 (secondi Unix), "file_path": "/"+remote}`. File `<key>.json`, key D14. Scrittura atomica: file temporaneo nella stessa dir (`os.CreateTemp`, permessi 0600) + `os.Rename`.
  - Media = estensione lowercase in {mp4, mov, avi, wmv, flv, webm, mkv, mp3, ogg, wav, m4a}.
  - `?resume-get` (GET/HEAD): header `Cache-Control: no-store`, `Content-Type: application/json`. Store off → 404 `{"error":"Playback resume disabled"}`; record assente → 404 `{"error":"No resume position found"}`; errore lettura/parse → 500 `{"error":"Failed to load position"}`; trovato → 200 record.
  - `?resume-save=<pos>[&duration=<d>]`: store off → 404 come sopra; `pos` non parsabile, non finito o < 0 → 400 `Invalid position value`; `duration` non parsabile o non finito → null; il target deve esistere come file media (`Stat` ok, non cartella, estensione media) altrimenti 404; salva → 204; errore → 500 `{"error":"Failed to save position"}`.
  - `?resume-clear`: store off → 404; rimuove il file (assente = ok) → 204; errore → 500 `{"error":"Failed to clear position"}`.
  - Listing (0.2): con store attivo `playback_resume_enabled=true`; per ogni file media con record → `resume_position` = `int64(position)` (troncato). Lettura per item: solo file media.
  - Pulizia: al più una volta ogni 24h (timestamp in memoria, mutex), alla prima save dopo la scadenza: in goroutine rimuove i `*.json` con `timestamp` (o mtime file se il parse fallisce) più vecchio di `Opt.ResumeRetention`.
  - VFS per la chiave: `cfg.GetVFS(r.Context())`.
  Scope: nessun dato fuori dalla resume dir; nessuna scrittura sul remote.
  Steps:
  1. S1 — `resume.go` (store: `load`, `save`, `clear`, `cleanup`, `key`), test unitari; expected G-U13a verde.
  2. S2 — endpoint + listing + README; e2e; expected G-U13b verde.
  Recovery boundary: none.
  Failure handling: `GetCacheDir` non scrivibile nei test → usa sempre `ResumeDir = t.TempDir()`.
- **Unit tests:** `webui/resume_test.go`, gate G-U13a: `TestResumeStoreRoundTrip` (save → load uguale, file 0600, dir 0700); `TestResumeKeyDistinct` (ConfigString diversi → chiavi diverse); `TestResumeCleanup` (record con timestamp vecchio rimosso, recente tenuto); `TestIsMedia`.
- **e2e tests:** gate G-U13b: `TestWebUIResumeEndpoints`: `Resume=true`, `ResumeDir=t.TempDir()`; file `v.mp4`; `?resume-get` → 404 + `Cache-Control: no-store`; `?resume-save=12.5&duration=100` → 204; `?resume-get` → 200 `position==12.5`, `duration==100`, `file_path=="/v.mp4"`; listing JSON → `playback_resume_enabled==true`, `v.mp4` `resume_position==12`; `?resume-save=-1` → 400; `?resume-save=1` su `x.txt` → 404; `?resume-clear` → 204, poi get → 404. Con `Resume=false` → get 404 `Playback resume disabled`, listing `playback_resume_enabled==false`.
- **Done:** G-U13a, G-U13b verdi; README sezione "Ripresa riproduzione"; unit chiusa; commit `sub-phase(1.3)`.

## Phase gates and closure
- Required gates: G-PHASE, G-FOOT, G-LINT.
- README obligation: 1.3 aggiunge "Ripresa riproduzione"; a P1 aggiungi/verifica sezione "Comportamento con client WebDAV" (D8, D9, D10: Origin, 412, append e cache mode).
- Every sub-phase DONE. Run full phase gates now.
- Supervisor Opus reviews: I-4, I-6, D9/D10 sui test, README.
- Open closure unit P1, record review and result, mark phase DONE, commit `phase(1)`.
