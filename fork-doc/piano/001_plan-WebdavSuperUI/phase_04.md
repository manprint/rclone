# Phase 3 — Link di download con token e link pubblici

Intent: con autenticazione attiva la UI genera link di download diretti con token a scadenza (file e zip di
cartelle) utilizzabili senza credenziali; per i percorsi autorizzati con `--webui-publink-paths` la UI
mostra link pubblici permanenti verso file e cartelle (viewer in sola lettura).
Prerequisites: P2 DONE.
Phase closure: P3; review by Opus (agent-1).

## State and ownership contract
Read STATE.md §0–1 first for scope, ownership, recovery, checks, and commit rules.
Open before edits; checkpoint at a recovery boundary; close with evidence. A
delegated worker follows its assigned OPEN unit. Missing design goes to the
configured supervisor.

## Local design context
Plan revision: 1.
- Chiavi: `master = sha256(Opt.Secret)` se `Opt.Secret != ""`, altrimenti 32 byte da `crypto/rand` generati in `New` (link non stabili tra riavvii: documentarlo). `tokKey = HMAC-SHA256(master, "webui-token")`, `pubKey = HMAC-SHA256(master, "webui-publink")`. Mai loggare chiavi o token completi.
- D12 token download: `token = strconv.FormatInt(expMs,10) + "." + base64.RawURLEncoding(user) + "." + hex(HMAC-SHA256(tokKey, user + "\x00" + remote + "\x00" + strconv.FormatInt(expMs,10))[:16])`; `expMs = now + Opt.TokenExpiry` in millisecondi Unix. Valido sse 3 parti, `expMs` intero, `now <= expMs`, user decodificabile, firma uguale (`hmac.Equal` sui byte hex). Legato al `remote` esatto (un token di cartella NON vale per i figli).
- D13 link pubblici: abilitati sse `len(Opt.PublinkPaths) > 0 && !cfg.IsProxy`. `allowed(remote)`: per ogni voce `e := strings.Trim(entry, "/")`: `e == ""` (root = tutto) oppure `remote == e` oppure `strings.HasPrefix(remote, e+"/")`. `pubToken(remote) = hex(HMAC-SHA256(pubKey, remote)[:16])`. Un token di cartella copre il sottoalbero: valido per `remote` se uguale a `pubToken(p)` per `p` = `remote` o uno dei suoi antenati, inclusa la root `""`.
- I-5: senza credenziali la PreAuth serve solo: asset; GET/HEAD `/@public/` con token valido; GET/HEAD `?token=` valido su file, o su cartella con `zip`. Tutto il resto → `next` (auth upstream → 401). Guardie: `TestPreAuthBoundaries`, `TestPublinkBoundaries`.
- I-4: path dopo il token passa da `cfg.CheckPath`; errore → 404.
- R2: la PreAuth gira prima di StripPrefix/CORS/auth: rimuove da sé il baseurl e avvolge le risposte con `u.publicWrap` (CORS + `--header`).
- Il servizio di token e link pubblici usa la VFS non-proxy (`cfg.GetVFS(r.Context())` funziona senza utente solo fuori dalla modalità proxy, per questo entrambi sono disattivati con `--auth-proxy`).

### Servizio file comune (`serveFileRaw(w, r, VFS, remote)`, in `webui/files.go`)
`node := VFS.Stat(remote)` (file); `h := node.Open(os.O_RDONLY)` → `defer h.Close()`; header D15 (CSP sandbox) per estensioni attive; `http.ServeContent(w, r, path.Base(remote), node.ModTime(), h)` (gestisce Range, HEAD, If-Modified-Since). Errore apertura → 404/500 come mappa errori 1.1.

## Sub-phases

### 3.1 Chiavi, `?tokengen`, download con `?token=` in PreAuth
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; Opus review focalizzata sicurezza (bypass auth, HMAC, confronto a tempo costante, I-5) prima della chiusura.
- **Files:**
  READ `webui/ui.go` (`PreAuth`, `Handler`, `New`), `webui/zip.go` (2.1), `webui/util.go`, `lib/http/server.go` (`UsingAuth`, ordine middleware), `cmd/serve/webdav/webdav_test.go` (auth helper).
  WRITE `webui/ui.go` (chiavi in `New`, dispatch `tokengen` in `Handler`, ramo token in `PreAuth`), test.
  NEW `webui/token.go`, `webui/token_test.go`, `webui/files.go`.
- **Change:**
  Preconditions: P2 DONE.
  Contract:
  - `func (u *UI) makeToken(user, remote string, now time.Time) string`; `func (u *UI) checkToken(tok, remote string, now time.Time) (user string, ok bool)`.
  - `Handler`, GET su nodo esistente con query `tokengen` (prima di edit/view/listing): 200 `text/plain; charset=utf-8`, `Cache-Control: no-store`, body = `makeToken(u.user(r), remote, time.Now())`. Nodo inesistente → next.
  - `PreAuth`, dopo gli asset: se metodo GET/HEAD e `r.URL.Query().Has("token")` e `cfg.UsingAuth()` e `!cfg.IsProxy` e header `Authorization` assente: `rel` (path senza baseurl); `CheckPath(rel)` errore → next; `checkToken(token, remoteFromPath(rel), now)` non valido → next. Valido: `VFS.Stat`; file → `serveFileRaw`; cartella con `zip` → stessa logica 2.1 (`?zip` o `?zip=multi`, incluse validazioni e `DisableZip`); cartella senza `zip` → next. Log `fs.Infof(remote, "webui: token download for user %q", user)`. Risposta avvolta da `publicWrap`.
  Scope: con auth disattivata il ramo token non si attiva (upstream serve già tutto); nessun effetto su metodi diversi da GET/HEAD.
  Steps:
  1. S1 — `token.go` (chiavi, make/check), `files.go`; test unitari; expected G-U31a verde.
  2. S2 — dispatch Handler/PreAuth; e2e; expected G-U31b verde.
  Recovery boundary: none.
  Failure handling: se l'auth upstream non vede la richiesta come attesa (es. 200 dove serve 401) → stop, escalation Opus con output.
- **Unit tests:** `webui/token_test.go`, gate G-U31a: `TestTokenRoundTrip` (make → check ok, user restituito); `TestTokenRejects` (scaduto, firma alterata di 1 carattere, user alterato, remote diverso, 2 parti, exp non numerico, base64 invalido); `TestKeysFromSecret` (stesso secret → stesso token; secret diversi → token diversi; secret vuoto → due `New` diversi producono token diversi); `TestPreAuthTokenIgnoredInProxy` (`cfg.IsProxy=true`, token valido → `next` chiamato).
- **e2e tests:** gate G-U31b, `TestPreAuthBoundaries` (server con basic auth `user`/`pass`, file `f.txt`, `g.txt`, cartella `d/` con `x.txt`):
  - GET asset senza credenziali → 200.
  - GET `/f.txt` senza credenziali → 401.
  - GET `/f.txt?tokengen` con credenziali → 200 token; GET `/f.txt?token=<tok>` senza credenziali → 200 e contenuto; HEAD idem → 200.
  - stesso token su `/g.txt?token=` → 401; PUT `/f.txt?token=<tok>` senza credenziali → 401.
  - `/d/?tokengen` con credenziali → token; `/d/?zip&token=<tok>` senza credenziali → 200 zip; `/d/?token=<tok>` (senza zip) → 401; `/d/x.txt?token=<tok di d>` → 401.
  - GET `/f.txt?token=<tok>` con `Authorization: Basic` errato → 401.
  - `TokenExpiry=1ms`: token generato, attesa 20ms → 401.
  - baseurl `/pfx`: `/pfx/f.txt?token=<tok>` → 200.
- **Done:** G-U31a, G-U31b verdi; review sicurezza Opus registrata; unit chiusa; commit `sub-phase(3.1)`.

### 3.2 Link pubblici: token nel listing e `/@public/`
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; Opus review focalizzata sicurezza (I-4, I-5, copertura sottoalbero) prima della chiusura.
- **Files:**
  READ `webui/token.go`, `webui/listing.go`, `webui/edit.go`, `webui/resume.go`, `webui/zip.go`, `webui/search.go`, superdufs `src/server.rs` (`serve_publink_dir`, ramo `@public/` in `handle`), superdufs `assets/index.js` (righe ~517–547: costruzione URL pubblico).
  WRITE `webui/ui.go` (ramo `/@public/` in `PreAuth`), `webui/listing.go` (`publink_token`, `publink_enabled`), `webui/search.go` (token sui risultati), `webui/README.md` (sezione "Link di download e link pubblici"), test.
  NEW `webui/public.go`, `webui/public_test.go`.
- **Change:**
  Preconditions: 3.1 DONE.
  Contract:
  - Listing/ricerca (link abilitati): per ogni item con `allowed(itemRemote)` → `PublinkToken = pubToken(itemRemote)` (`itemRemote = path.Join(dirRemote, name)`). `PublinkEnabled = allowed(dirRemote) || qualche item ha token`. Link disabilitati → nessun token, `false`.
  - `PreAuth`, dopo gli asset e prima del ramo token: se link abilitati e `rel` ha prefisso `/@public/`: metodi diversi da GET/HEAD → next. `rest := strings.TrimPrefix(rel, "/@public/")`; `tok, sub, found := strings.Cut(rest, "/")`; `!found` o `tok == ""` → 404. `p := "/" + sub`; `CheckPath(p)` errore → 404. `remote := remoteFromPath(p)`. Token non valido per `remote` (D13, antenati) → 404. `VFS.Stat(remote)` errore → 404. `!allowed(remote)` → 403. Tutte le risposte avvolte da `publicWrap`.
  - Cartella: GET senza slash finale (`!strings.HasSuffix(p, "/")` e `remote != ""`) → 302 a `baseNoSlash + "/@public/" + tok + "/" + remote + "/"` (+ query). `zip` → 2.1 (nome zip come 2.1). Altrimenti IndexData viewer: `Kind "Index"`, `Href = "/" + remote` (root `"/"`), `URIPrefix = baseSlash + "@public/" + tok + "/"`, tutti i booleani `false` tranne `DirExists=true`, `User=nil`, `Paths` = figli con `allowed(childRemote)`, senza `PublinkToken`, `Size` D11, ordinati con `sort`/`order`. `json` → JSON, altrimenti HTML (stesso render 0.2).
  - File: `resume-get|resume-save|resume-clear` → handler 1.3 sul `remote`; `view` → pagina EditData `Kind "View"` con `Href = "/" + remote`, `URIPrefix = baseSlash`, `AllowUpload/AllowDelete/Auth=false`, `User=nil`, `Editable` calcolato come 1.2; altrimenti `serveFileRaw`.
  - Con link disabilitati la PreAuth non intercetta `/@public/` (una cartella reale `@public` resta raggiungibile). Documentare che con link abilitati il nome `@public` alla root è riservato.
  Scope: nessuna scrittura possibile via `/@public/`.
  Steps:
  1. S1 — `public.go` (`allowed`, `pubToken`, `checkPubToken`, `servePublic`), test unitari; expected G-U32a verde.
  2. S2 — token nel listing/ricerca, dispatch PreAuth, README; e2e; expected G-U32b verde.
  Recovery boundary: none.
  Failure handling: comportamento JS del viewer non coerente con `URIPrefix` (link rotti nel breadcrumb) → registra il caso e chiedi a Opus.
- **Unit tests:** `webui/public_test.go`, gate G-U32a: `TestPublinkAllowed` (voci `pub`, `/a/b/`, `/`; casi `pub`, `pub/x`, `pubx` → false, `a/b/c`, root); `TestPubTokenSubtree` (token di `pub` valido per `pub/sub/f`, non per `priv/f` né per `pubx/f`; token di file non valido per un altro file); `TestPublicIgnoredInProxy` (`IsProxy=true` → `next`); `TestPublicForbiddenWhenNotAllowed` (token valido per `old` ma `PublinkPaths=["new"]` → 403).
- **e2e tests:** gate G-U32b, `TestPublinkBoundaries` (basic auth attiva, `PublinkPaths=["pub"]`, file `pub/a.txt`, `pub/sub/b.txt`, `priv/c.txt`):
  - listing root (con credenziali) → `pub` ha `publink_token`, `priv` no, `publink_enabled==true`; listing `/pub/` → `a.txt` con token.
  - senza credenziali: `/@public/<tok pub>/pub/sub/b.txt` → 200 contenuto; `/@public/<tok pub>/priv/c.txt` → 404; `/@public/<tok a.txt>/pub/sub/b.txt` → 404; `/@public/bad/pub/a.txt` → 404; PUT `/@public/<tok pub>/pub/a.txt` → 401.
  - `/@public/<tok pub>/pub` → 302 a `/@public/<tok pub>/pub/`; `/@public/<tok pub>/pub/?json` → `uri_prefix=="/@public/<tok>/"`, `href=="/pub"`, `allow_upload==false`, nessun `publink_token` negli item; `/@public/<tok pub>/pub/?zip` → zip.
  - `/@public/<tok pub>/pub/a.txt?view` → html con EditData `kind=="View"`.
  - baseurl `/pfx`: `/pfx/@public/<tok>/pub/a.txt` → 200.
  - `TestPublicPathTraversal`: `/@public/<tok pub>/pub/%2e%2e/priv/c.txt` → 404.
  - `PublinkPaths` vuoto → listing senza token, `publink_enabled==false`, `/@public/...` senza credenziali → 401.
- **Done:** G-U32a, G-U32b verdi; review sicurezza Opus registrata; README aggiornato; unit chiusa; commit `sub-phase(3.2)`.

## Phase gates and closure
- Required gates: G-PHASE, G-FOOT, G-LINT.
- README obligation: 3.2 "Link di download e link pubblici" (formato, scadenza, `--webui-secret` per link stabili, disattivazione con `--auth-proxy`, nome riservato `@public`).
- Every sub-phase DONE. Run full phase gates now.
- Supervisor Opus reviews: I-4, I-5 (tutti i casi negativi), gestione chiavi, log senza segreti.
- Open closure unit P3, record review and result, mark phase DONE, commit `phase(3)`.
