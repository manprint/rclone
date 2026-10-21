# Phase 4 — Blindatura del contratto, e2e su binario reale, documentazione

Intent: un aggiornamento degli asset superdufs che introduce endpoint non gestiti fa fallire i test; lo
scenario di riferimento (`rclone serve webdav --addr 127.0.0.1:8000 <dir>`) è verificato sul binario reale;
il fork è documentato con le procedure di sync da upstream e superdufs.
Prerequisites: P3 DONE.
Phase closure: P4 (finale); review by Opus (agent-1).

## State and ownership contract
Read STATE.md §0–1 first for scope, ownership, recovery, checks, and commit rules.
Open before edits; checkpoint at a recovery boundary; close with evidence. A
delegated worker follows its assigned OPEN unit. Missing design goes to the
configured supervisor.

## Local design context
Plan revision: 1.
- I-7: gli asset restano byte-identici a `SOURCE`; l'unico adattamento JS è `overlay.js`.
- API surface lock: l'insieme di parametri query e metodi HTTP usati da `webui/assets/index.js` è confrontato con un insieme atteso gestito dal server; differenza → test rosso con messaggio che elenca i nomi nuovi/spariti e rimanda a `webui/README.md` §Sincronizzazione.
- RS (reference scenario): binario compilato; `serve webdav` su una dir temporanea; browser-like GET `/` → pagina superdufs; curl → listing upstream; PUT con Origin in sottocartella nuova → 201; `?json`, `?zip`; MOVE su cartella esistente con Origin → 412; con `--webui=false` nessuna pagina superdufs.
- D17: documentazione del fork solo in `webui/README.md` (italiano).

## Sub-phases

### 4.1 API surface lock, test overlay in node, smoke e2e
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P4.
- **Files:**
  READ `webui/assets/index.js` (tutte le occorrenze di `searchParams.set(`, `"?…`, `&…=`, `method:`, `xhr.open(`), `webui/ui.go` (dispatch completo), superdufs `tests/frontend/harness.mjs` (solo come riferimento per caricare `index.js` in `node:vm`).
  NEW `webui/surface_test.go`, `webui/overlay/overlay_test.mjs`, `webui/overlay_node_test.go`, `webui/scripts/smoke.sh`.
- **Change:**
  Preconditions: P3 DONE.
  Contract:
  - `TestAPISurfaceLock`: legge l'`index.js` embedded; estrae (a) nomi query con le regex `searchParams\.set\("([a-z-]+)"`, `[?&]([a-z][a-z-]*)(?:=|\b)` dentro letterali stringa/template che iniziano con `?` o contengono `?`/`&` subito prima del nome, (b) metodi con `method:\s*"([A-Z]+)"` e `\.open\("([A-Z]+)"`. Prima di fissare le regex, stampa l'insieme estratto dall'`index.js` attuale e verificalo a mano contro l'elenco atteso: query {`zip`, `paths`, `q`, `edit`, `view`, `playlist`, `tokengen`, `token`, `disk-stats`, `resume-get`, `resume-save`, `duration`, `resume-clear`, `json`, `login`, `sort`, `order`} e metodi {`HEAD`, `PUT`, `PATCH`, `DELETE`, `MKCOL`, `MOVE`, `CHECKAUTH`, `LOGOUT`, `GET`}. Se l'estrazione trova nomi non in elenco, valuta: nome gestito dal server → aggiungilo all'atteso con commento; nome non gestito → escalation Opus. L'atteso è una costante nel test.
  - `overlay_test.mjs` (eseguito con `node --test`): crea un contesto `vm` con stub minimi (`window`/`document` con `addEventListener` no-op, `location`, `localStorage` in memoria — vedi approccio di `harness.mjs`), esegue `index.js` poi `overlay.js` (percorsi relativi al file di test), verifica `formatDirSizeParts(-1)` → `["—", ""]`, `formatDirSizeParts(null)` → `["—", ""]`, e per `0`, `1`, `5` lo stesso risultato della funzione originale (catturata prima di eseguire l'overlay).
  - `TestOverlayNode` (Go): `exec.LookPath("node")` assente → `t.Skip`; altrimenti `node --check` su `assets/index.js` e `overlay/overlay.js`, poi `node --test overlay/overlay_test.mjs`; exit ≠ 0 → fail con output.
  - `smoke.sh` (bash, `set -euo pipefail`, da root repo): `go build -o "$TMP/rclone" .` con `TMP=$(mktemp -d)` e `trap` di pulizia (kill server, rm); porta libera (`python3 -c` socket bind 0 oppure range fisso 18000+$$%1000); avvia `RCLONE_CONFIG=/notfound "$TMP/rclone" serve webdav --addr 127.0.0.1:$PORT "$TMP/root"`; attende fino a 10s la porta; esegue i controlli RS con `curl -s -o -w '%{http_code}'`; poi riavvia con `--webui=false` e verifica assenza di `index-data`. Stampa `smoke OK` ed esce 0; ogni controllo fallito stampa atteso/ottenuto ed esce 1.
  Scope: nessun cambiamento al codice di produzione salvo bug trovati (in quel caso: fix minimo + test, annotato nel ledger).
  Steps:
  1. S1 — `surface_test.go` (estrazione verificata manualmente); expected test verde.
  2. S2 — `overlay_test.mjs`, `overlay_node_test.go`; expected verde (o skip motivato se node assente: qui node v24 è presente, quindi deve girare).
  3. S3 — `smoke.sh`; expected `smoke OK`.
  Recovery boundary: none.
  Failure handling: smoke fallisce per un bug di produzione → fix minimo nell'unità con test di regressione; bug di design → escalation Opus.
- **Unit tests:** `TestAPISurfaceLock`, `TestOverlayNode` (+ `overlay_test.mjs`), gate G-U41 (discovery: entrambi RUN/PASS, `TestOverlayNode` non SKIP in questo ambiente).
- **e2e tests:** `webui/scripts/smoke.sh`, gate G-SMOKE (output `smoke OK`).
- **Done:** G-U41 e G-SMOKE verdi; unit chiusa; commit `sub-phase(4.1)`.

### 4.2 Documentazione del fork e script di manutenzione
- **Model:** Sonnet (agent-2)
- **Assignment:** implementa; review Opus a P4.
- **Files:**
  READ `webui/README.md`, `webui/scripts/*.sh`, `Makefile` (target `rclone`, LDFLAGS versione), `overview.md` del piano.
  WRITE `webui/README.md`.
  NEW `webui/scripts/sync-upstream.sh`, `webui/scripts/build.sh`.
- **Change:**
  Preconditions: 4.1 DONE.
  Contract:
  - `sync-upstream.sh` (bash, `set -euo pipefail`): richiede working tree pulito; `REMOTE=${UPSTREAM_REMOTE:-origin}`, `BRANCH=${UPSTREAM_BRANCH:-master}`; `git fetch "$REMOTE"`; `git -c rerere.enabled=true merge --no-edit "$REMOTE/$BRANCH"`; in caso di conflitto stampa i file in conflitto e le istruzioni (le 3 righe `// fork:webui` vanno ri-aggiunte) ed esce 1; dopo il merge esegue `webui/scripts/check-footprint.sh`, `go build ./...`, `go test ./webui/... ./cmd/serve/webdav/ ./lib/http/...` e `RCLONE_WEBUI=false go test ./cmd/serve/webdav/`. Non fa push.
  - `build.sh`: `go build -ldflags "-s -X github.com/rclone/rclone/fs.Version=$(git describe --tags --always --dirty)-webui" -o rclone .` (verifica il nome del simbolo versione in `fs/version.go` o nel `Makefile` prima di scriverlo).
  - `webui/README.md` completo (italiano): Scopo; Avvio rapido (`rclone serve webdav --addr 0.0.0.0:8000 myremote:`); Flag (tabella completa con default, più flag upstream che influenzano la UI: `--read-only`, `--disable-zip`, `--template`, `--disable-dir-list`, `--baseurl`, `--user/--pass/--htpasswd/--auth-proxy`, `--vfs-cache-mode`, `--no-modtime`); Interfaccia web; Comportamento con client WebDAV; Ripresa riproduzione; Ricerca e statistiche; Link di download e link pubblici; Sicurezza (CSP, Origin, `@public`, secret); Limiti noti (append richiede cache, conteggio cartelle, link non stabili senza secret, mtime delle cartelle); Architettura del fork (file, 3 righe in `webdav.go`, overlay); Sincronizzazione con upstream (`sync-upstream.sh`, rerere, footprint) e con superdufs (`sync-assets.sh`, API surface lock); Test (gate e smoke).
  Scope: nessun file upstream di documentazione modificato (D2, D17).
  Steps:
  1. S1 — script; `bash -n` su tutti gli script; `shellcheck` se disponibile; expected nessun errore.
  2. S2 — README; expected tutte le sezioni presenti e coerenti con i flag registrati (`rclone serve webdav --help | grep webui` confrontato con la tabella).
  Recovery boundary: none.
  Failure handling: none noto.
- **Unit tests:** N/A — script e documentazione; verifica: `bash -n webui/scripts/*.sh` (gate G-U42), confronto help/README.
- **e2e tests:** N/A — lo scenario è coperto da G-SMOKE (4.1); `build.sh` eseguito una volta e `./rclone version` mostra il suffisso `-webui`.
- **Done:** G-U42 verde, README completo e coerente, unit chiusa, commit `sub-phase(4.2)`.

## Phase gates and closure
- Required gates: G-FINAL (G-PHASE + `-race` + G-SMOKE), G-FOOT, G-LINT.
- README obligation: 4.2 (completo).
- Every sub-phase DONE. Run full final gates now.
- Supervisor Opus reviews: RS soddisfatto end-to-end, tutti gli invarianti I-1…I-7 con evidenza, README.
- Open closure unit P4, record review and result, mark phase and feature DONE, commit `phase(4)`.
- On final phase: RS = G-SMOKE + `TestWebUIListingHTML` + `TestWebUIDisabledFlag`; final gates G-FINAL.
