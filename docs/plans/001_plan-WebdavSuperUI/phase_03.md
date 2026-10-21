# Phase 2 — Archivi zip, ricerca, statistiche disco

Intent: dalla UI si scaricano cartelle e selezioni multiple come zip, si cerca per nome nel sottoalbero e
(se abilitato) si vedono le statistiche del disco.
Prerequisites: P1 DONE.
Phase closure: P2; review by Opus (agent-1).

## State and ownership contract
Read STATE.md §0–1 first for scope, ownership, recovery, checks, and commit rules.
Open before edits; checkpoint at a recovery boundary; close with evidence. A
delegated worker follows its assigned OPEN unit. Missing design goes to the
configured supervisor.

## Local design context
Plan revision: 1.
- D16: ricerca BFS sulla VFS limitata da `Opt.SearchMaxDepth`, `Opt.SearchMaxResults`, `Opt.SearchTimeout`; al raggiungimento di un limite si restituisce il risultato parziale (200).
- `cfg.DisableZip` (flag upstream `--disable-zip`) disattiva anche gli zip della UI.
- I-4: path della selezione validati come elementi relativi (nessun `..`, nessun path assoluto) e poi con `cfg.CheckPath("/" + joined)`.
- Gli zip si generano in streaming verso la risposta: un errore dopo l'inizio del body si può solo loggare (`fs.Errorf`), lo status è già 200.

### Dispatch aggiunto al `Handler` (cartella, GET/HEAD, prima del listing 0.2, dopo `download`)
Ordine: `zip` (valore `multi` → multi-zip, altro/vuoto → zip cartella) → `q` → `disk-stats` → `json`/HTML (0.2).
Le query dei passi successivi (`tokengen`, fase 3) non sono ancora gestite.

## Sub-phases

### 2.1 `?zip` e `?zip=multi`
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P2.
- **Files:**
  READ `vfs/zip.go` (`CreateZip` — struttura header, metodi Deflate/Store, nomi entry, modtime), `cmd/serve/webdav/webdav.go` `serveDir` (naming zip upstream: `path.Base(dirRemote)`, root → `root`), superdufs `src/server.rs` `handle_zip_selection` (riferimento validazioni), `webui/ui.go`.
  WRITE `webui/ui.go` (dispatch), test.
  NEW `webui/zip.go`, `webui/zip_test.go`.
- **Change:**
  Preconditions: P1 DONE.
  Contract:
  - Comuni: `cfg.DisableZip` → 403 `Zip downloads are disabled`. Header: `Content-Type: application/zip`, `Content-Disposition: mime.FormatMediaType("attachment", map[string]string{"filename": name})`, `Last-Modified` = ora UTC (come upstream). HEAD → solo header dopo la validazione.
  - `?zip` (valore vuoto o diverso da `multi`): `name = path.Base(dirRemote) + ".zip"`, root → `root.zip`; body = `vfs.CreateZip(ctx, dir, w)`; errore → `fs.Errorf(dirRemote, "webui: zip failed: %v", err)`.
  - `?zip=multi&paths=<json>`, validazioni nell'ordine (risposta `400` con body testuale esatto):
    1. `paths` assente o vuoto → `Missing paths parameter`.
    2. `len(paths) > 8192` → `Selection payload too large`.
    3. `json.Unmarshal` in `[]string` fallisce → `Invalid selection payload`.
    4. lista vuota → `Selection is empty`.
    5. `len > Opt.MultiZipMax` → `Too many selected paths`.
    6. per ogni elemento: `strings.Trim(s, "/")`; vuoto → salta; qualche segmento (split `/`) uguale a `..` o `.` o vuoto, o contiene `\` → `Invalid selection path`; deduplica mantenendo l'ordine.
    7. risultato vuoto → `Selection is empty`; `len > Opt.MultiZipMax` → `Too many selected paths`.
    8. per ogni elemento `full := path.Join(dirRemote, rel)`; `cfg.CheckPath("/"+full)` errore → `Invalid selection path`; `VFS.Stat(full)` errore → `404 Not Found`.
    Poi `name = "selection.zip"`; body = `writeSelectionZip(ctx, w, VFS, dirRemote, rels)`.
  - `writeSelectionZip`: `zip.NewWriter(w)`; per ogni elemento: file → entry con nome `rel` (path relativo alla cartella corrente, separatore `/`), header da `zip.FileInfoHeader` adattato come in `vfs/zip.go` (Deflate, `Modified` = modtime); cartella → entry `rel + "/"` (Store) e ricorsione su `ReadDirAll` con nomi `rel/child`. Controllo `ctx.Err()` a ogni entry. Rispecchia la struttura di `CreateZip` (leggerla e riusare le stesse scelte di header). Chiusura del writer alla fine.
  Scope: nessun cambiamento a `?download=zip` upstream (passa ancora a next tramite il passo 5 di 0.2).
  Steps:
  1. S1 — `zip.go` (`parseSelection(raw string, max int) ([]string, int, string)` che ritorna lista, status, messaggio; `writeSelectionZip`), test unitari; expected G-U21a verde.
  2. S2 — dispatch + e2e; expected G-U21b verde.
  Recovery boundary: none.
  Failure handling: none noto.
- **Unit tests:** `webui/zip_test.go`, gate G-U21a: `TestParseSelection` (tabella con ognuno dei messaggi 1–7, dedup, trim degli slash, `a/../b` → invalid, 201 elementi con max 200 → too many); `TestWriteSelectionZip` (VFS su `t.TempDir()` con `a.txt`, `d/b.txt`: zip contiene `a.txt`, `d/`, `d/b.txt` con contenuti corretti).
- **e2e tests:** gate G-U21b, `TestWebUIZip`: GET `/d/?zip` → 200, `Content-Disposition` con `d.zip`, zip valido con `b.txt`; GET `/?zip` → `root.zip`; `?zip=multi&paths=["a.txt","d"]` → `selection.zip` con `a.txt`, `d/b.txt`; HEAD stessa URL → 200 senza body; `paths=["../x"]` → 400 `Invalid selection path`; `paths=["nope"]` → 404; `DisableZip=true` (opzione upstream `opt.DisableZip`) → 403 e `allow_archive==false` nel listing.
- **Done:** G-U21a, G-U21b verdi; unit chiusa; commit `sub-phase(2.1)`.

### 2.2 Ricerca `?q` e `?disk-stats`
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P2.
- **Files:**
  READ `webui/listing.go`, `vfs/vfs.go` (`Statfs`), `vfs/dir.go` (`ReadDirAll`), superdufs `src/server.rs` `handle_search_dir` (riferimento).
  WRITE `webui/ui.go` (dispatch), `webui/listing.go` (riuso build IndexData con item forniti), `webui/README.md` (sezione "Ricerca e statistiche"), test.
  NEW `webui/search.go`, `webui/search_test.go`.
- **Change:**
  Preconditions: 2.1 DONE.
  Contract:
  - `?q=<s>`: `!Opt.Search` → ignorato (listing normale, `allow_search=false` già da 0.2). `s` dopo `strings.ToLower(strings.TrimSpace(...))` vuoto → listing normale. Altrimenti BFS dalla cartella corrente: coda di `(dir, depth)`; per ogni figlio: se `strings.Contains(strings.ToLower(name), s)` → `PathItem` con `Name` = path relativo alla cartella corrente (`a/b/c.txt`); le cartelle con `depth+1 <= Opt.SearchMaxDepth` vanno in coda. Stop quando i risultati raggiungono `Opt.SearchMaxResults` o scade il contesto `context.WithTimeout(r.Context(), time.Duration(Opt.SearchTimeout))`. Errori `ReadDirAll` di una sottocartella → log debug e continua. Risposta = stessa IndexData/HTML/JSON della cartella (stesso `href`), `Paths` = risultati ordinati con `sortPathItems`. Le cartelle trovate hanno `Size` secondo D11 senza conteggio (sempre -1, per costo).
  - `?disk-stats` (GET): `!Opt.DiskStats` → 404. `total, used, free := VFS.Statfs()`; ogni valore `< 0` → `null`; tutti e tre `< 0` → 404 `{"error":"Disk usage unknown"}`. Altrimenti 200 `application/json` `{"disk_usage": used, "disk_total": total, "disk_available": free}` con `Cache-Control: no-cache`.
  Scope: la ricerca non modifica il listing senza `q`.
  Steps:
  1. S1 — `search.go` (`search(ctx, dir *vfs.Dir, needle string, maxDepth, maxResults int) []PathItem`), test unitari; expected G-U22a verde.
  2. S2 — dispatch, disk-stats, README; e2e; expected G-U22b verde.
  Recovery boundary: none.
  Failure handling: none noto.
- **Unit tests:** `webui/search_test.go`, gate G-U22a: `TestSearchLimits`: albero `a/b/c/d/match.txt`, `match1.txt`, `x/match2.txt`; maxDepth 1 → solo `match1.txt` e `x/match2.txt`; maxResults 1 → 1 risultato; contesto già scaduto → 0 risultati senza errore; case-insensitive (`MATCH`).
- **e2e tests:** gate G-U22b: `TestWebUISearch`: `GET /?q=match&json` → `paths` con nomi relativi (`x/match2.txt`); `Search=false` → `allow_search==false` e `?q` restituisce il listing normale. `TestWebUIDiskStats`: `DiskStats=false` → 404; `DiskStats=true` su backend local → 200 con `disk_total > 0`; listing `enable_data_stats==true`.
- **Done:** G-U22a, G-U22b verdi; README aggiornato; unit chiusa; commit `sub-phase(2.2)`.

## Phase gates and closure
- Required gates: G-PHASE, G-FOOT, G-LINT.
- README obligation: 2.2 "Ricerca e statistiche"; a P2 verifica che la sezione "Flag" riporti `--disable-zip` come interruttore anche per gli zip della UI.
- Every sub-phase DONE. Run full phase gates now.
- Supervisor Opus reviews: validazioni selezione (I-4), limiti ricerca, streaming zip.
- Open closure unit P2, record review and result, mark phase DONE, commit `phase(2)`.
