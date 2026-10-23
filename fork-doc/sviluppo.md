# Sviluppo del fork

Come è fatto il fork, come si tiene allineato a rclone upstream e a superdufs e come si prova. La guida
d'uso è in [uso.md](uso.md), la build in [build.md](build.md), la CI in [ci-e-release.md](ci-e-release.md).

## Architettura del fork

### Dove sta il codice

| File | Contenuto |
|------|-----------|
| `webui/options.go` | I flag `--webui*` e il blocco di opzioni `webdav_webui` (anche per `rc`) |
| `webui/ui.go` | `UI`, `Config` e `New`; `PreAuth` e `Handler`, i due middleware, e il dispatch delle richieste |
| `webui/assets.go` | Incorpora gli asset, calcola l'hash del prefisso `__webui_<hash>__` e li serve con cache immutabile |
| `webui/listing.go` | Elenco di una cartella: pagina, `?json`, ordinamento, redirect delle cartelle senza slash, conteggio |
| `webui/data.go` | Strutture dei dati incorporati nella pagina (`IndexData`, `EditData`) e ordinamento naturale |
| `webui/edit.go` | `?edit` e `?view`, controllo dei file modificabili, sandbox dei file attivi |
| `webui/files.go` | Scarico di un file |
| `webui/write.go` | Scritture dei browser: `CHECKAUTH`, `LOGOUT`, `PUT`, `PATCH` append, guardia del `MOVE`; rifiuti di `--webui-allow-delete=false` per tutti i client |
| `webui/zip.go` | Scarico come zip di una cartella o di una selezione |
| `webui/search.go` | Ricerca ricorsiva (`?q`) e statistiche di spazio (`?disk-stats`) |
| `webui/resume.go` | Posizioni di riproduzione (`?resume-get`, `?resume-save`, `?resume-clear`) |
| `webui/token.go` | Chiavi derivate dal segreto, token di download (`?tokengen`, `?token`) |
| `webui/public.go` | Link pubblici `@public` |
| `webui/util.go` | Funzioni comuni: intestazione `Origin`, utente, percorsi di destinazione, errori e loro codice HTTP |
| `webui/lookup.go` | Stat ed elenchi della VFS condivisi tra richieste contemporanee e abbandonati quando la richiesta finisce |
| `webui/assets/` | Asset di superdufs, byte per byte, e `SOURCE` (provenienza) |
| `webui/overlay/overlay.js` | Le sole correzioni JavaScript del fork, caricate dopo `index.js` |
| `webui/scripts/` | `check-footprint.sh`, `sync-assets.sh`, `sync-upstream.sh`, `smoke.sh`, `build.sh`, `gen-ci.py` |
| `.github/workflows/fork-*.yml` | CI del fork, generata da `gen-ci.py` a partire dai workflow upstream |
| `webui/justfile`, `webui/docker/` | Recipe `just` e `Dockerfile` per l'immagine Docker e per i binari compilati in Docker |
| `cmd/serve/webdav/webdav_webui.go` | Il collegamento tra `serve webdav` e il package: registra i flag, crea la UI se le condizioni lo permettono e la aggancia al server |

Il package `webui` non conosce `serve webdav`: riceve tutto dal glue attraverso `webui.Config` (VFS,
controllo dei percorsi, autenticazione, realm, `--disable-zip`, CORS e intestazioni di risposta). Così le
modifiche upstream a `serve webdav` toccano solo il glue.

### Le tre righe in `webdav.go`

L'unica modifica a un file upstream, in `cmd/serve/webdav/webdav.go`, sono tre righe aggiunte, ognuna col
commento `// fork:webui` e nessuna riga upstream cambiata:

1. il campo `webui *webuiGlue` nella struttura `WebDAV`;
2. l'opzione `w.webuiPreAuth()` nella chiamata a `libhttp.NewServer`;
3. la chiamata `w.webuiUse(router)` dopo gli altri middleware e prima di `router.Handle`.

`check-footprint.sh` fallisce se il fork tocca altri file upstream, se `webdav.go` ha righe tolte o
cambiate, o più di tre righe aggiunte.

### PreAuth e Handler

```
richiesta -> [PreAuth] -> StripPrefix -> CORS -> intestazioni -> AUTH -> VFS -> gzip
          -> [Handler] -> WebDAV upstream
```

`PreAuth` è registrato come opzione di `libhttp.NewServer`, quindi è il più esterno e vede le richieste prima
dell'autenticazione: serve soltanto asset, link pubblici validi e token di download validi; tutto il resto
passa avanti. Per questo riapplica da sé CORS e intestazioni di risposta. `Handler` è registrato come
ultimo middleware prima delle rotte e serve, già autenticato, le pagine, le scritture dei browser, la
ricerca e il resto; ciò che non è interfaccia passa al server WebDAV upstream. Con
`--webui-allow-delete=false` è anche il punto in cui `DELETE`, `MOVE` e le sovrascritture di qualunque
client sono rifiutate, prima di ogni altro controllo (`refuseDelete`, `TestWebUIAllowDeleteOff`). Questo ordine dipende da
come `libhttp.NewServer` registra le opzioni e i middleware: lo proteggono `TestPreAuthBoundaries` e
`TestPublinkBoundaries`, da guardare dopo ogni sincronizzazione con upstream.

### Asset e overlay

Gli asset di superdufs sono incorporati senza modifiche: `TestAssetsMatchSource` confronta i loro sha256 con
`webui/assets/SOURCE`. L'unico adattamento JavaScript è `webui/overlay/overlay.js` (oggi, la dimensione delle cartelle
sconosciuta mostrata come trattino), iniettato nella pagina subito dopo il tag di `index.js`. Se il template
cambia e quel tag non compare esattamente una volta, `webui.New` fallisce: il server parte, scrive
"webui disabled" nel log e si comporta come upstream.

### Perché lo zip non usa `vfs.CreateZip`

`vfs.CreateZip` dà alle cartelle oltre il secondo livello un nome che ripete il prefisso (concatena la radice
e il percorso completo della cartella): un archivio con cartelle annidate ha nomi sbagliati. `webui/zip.go`
costruisce da sé l'archivio, con le stesse scelte di compressione (Deflate per i file, Store per le
cartelle). Correggere `vfs.CreateZip` sarebbe una modifica a un file upstream, vietata dal footprint.

## Sincronizzazione

### Con upstream rclone

Il fork vive sul ramo `main` di https://github.com/manprint/rclone (fork GitHub di rclone/rclone), che
segue `master` di upstream. I remote sono `origin` = il fork e `upstream` = rclone/rclone. Per aggiornarlo,
con l'albero di lavoro pulito (i file non tracciati sono ammessi):

```bash
git clone https://github.com/manprint/rclone.git && cd rclone       # una volta sola
git remote add upstream https://github.com/rclone/rclone.git        # una volta sola
git config rerere.enabled true                                      # una volta sola, vedi sotto
webui/scripts/sync-upstream.sh                                      # merge di upstream/master in main
git push origin main                                                # quando lo script stampa OK
```

Lo script, che non fa mai push, esegue nell'ordine: `git fetch`, `git merge --no-edit upstream/master` con
`rerere` acceso, `webui/scripts/gen-ci.py` (rigenera la [CI del fork](ci-e-release.md) e la committa se
cambia) e poi i controlli `webui/scripts/check-footprint.sh` (confronto con il ramo appena unito),
`go build ./...`, `go test ./webui/... ./cmd/serve/webdav/ ./lib/http/...` e
`RCLONE_WEBUI=false go test ./cmd/serve/webdav/`. Stampa `OK` solo se tutto passa. Se non esiste il
remote `upstream` usa `origin`. Con `UPSTREAM_REMOTE` e `UPSTREAM_BRANCH` si scelgono un altro remote
e un altro ramo, con `UPSTREAM_REF` un riferimento qualsiasi, per esempio il tag di una release
(`UPSTREAM_REF=v1.76.0`).

`rerere` ricorda come è stato risolto un conflitto e lo riapplica al conflitto successivo: tienilo
acceso (`git config rerere.enabled true`), il conflitto è sempre lo stesso.

In caso di conflitto lo script elenca i file in conflitto ed esce con `1`, senza interrompere il merge. Per
risolvere: tieni il codice upstream; in `cmd/serve/webdav/webdav.go` riaggiungi le tre righe `// fork:webui`
descritte sopra; poi `git add`, `git commit` e rilancia lo script (il merge è già fatto, esegue solo i
controlli). Per rinunciare: `git merge --abort`.

Dopo ogni sincronizzazione, se `go test` fallisce nei test di PreAuth, upstream ha cambiato l'ordine dei
middleware o delle opzioni di `libhttp.NewServer`: va adattato il glue, non gli asset.

### Backport da upstream

Una correzione upstream non ancora unita (una PR aperta, o una patch del fork proposta a upstream) si
porta nel fork come backport: un commit che contiene la correzione e la sua patch in
`webui/backports/NN-<nome>.patch`, generata con `git format-patch` dal commit stesso prima di aggiungerla.
`git revert` di quel commit toglie il backport per intero.

- `check-footprint.sh` costruisce in un indice temporaneo la base upstream con le patch applicate, in
  ordine di nome, e confronta il fork con quella: un file upstream toccato da un backport deve coincidere
  con la patch, e le regole di `webdav.go` valgono come prima. Fallisce se una patch non si applica più
  alla base, o se due patch toccano lo stesso file.
- `sync-upstream.sh`, dopo il merge, cancella e committa le patch che si annullano all'indietro sulla
  versione upstream appena unita, cioè quelle che upstream contiene già identiche.
- Se upstream unisce la correzione modificata o ne fa un'altra, il merge dà conflitto sui file del
  backport e lo script dice quale patch li tocca: si tiene la versione upstream del file e si fa
  `git rm` della patch. Se invece upstream ha solo cambiato righe vicine, si rifà il backport sul nuovo
  codice.
- Una PR si porta solo se il suo diff (`git diff`, che funziona anche con i merge nel ramo della PR) si
  applica senza modifiche alla base del fork: adattata a mano non
  coinciderebbe più con quella che upstream unirà, e non sparirebbe da sola.

```bash
git fetch upstream pull/<N>/head:pr-<N>
git diff "$(git merge-base upstream/master pr-<N>)" pr-<N> | git apply --index
git commit --author="<autore della PR>"   # "task(<unità>): <titolo della PR> (backport)", nel corpo
                                         # "Backport of rclone/rclone#<N> (head <sha>)" e i messaggi upstream
git format-patch -1 --stdout HEAD > webui/backports/<NN>-pr<N>.patch
git add webui/backports && git commit --amend --no-edit
webui/scripts/check-footprint.sh
git branch -D pr-<N>
```

L'elenco dei backport è la cartella `webui/backports/`: ogni patch inizia con il messaggio del suo commit,
che dice cosa corregge e da quale PR viene (`grep -h '^Subject:' webui/backports/*.patch`). Quando una patch
sparisce perché upstream l'ha unita, `sync-upstream.sh` ricorda di aggiornare i documenti che la citano
(`grep -rn <nome> fork-doc docker-serve-webdav`).

Una PR non si porta, o si toglie con `git revert`, se non si applica alla base del fork o se rompe un test
upstream: in quel caso va aggiornata upstream dal suo autore.

### Con superdufs

Gli asset (`webui/assets/index.html`, `index.js`, `index.css`, `favicon.ico`) sono copiati byte per byte da
superdufs; `webui/overlay/overlay.js` contiene le sole correzioni del fork e viene caricato dopo `index.js`.
`webui/assets/SOURCE` registra repository, commit e sha256 degli asset. Come repository scrive
"superdufs, fork of https://github.com/sigoden/dufs" e non l'URL della copia locale, che può stare su un
server privato; `SUPERDUFS_REPO=<url pubblico>` lo sostituisce.

Per aggiornare gli asset da una copia locale di superdufs:

```bash
webui/scripts/sync-assets.sh /percorso/a/superdufs
```

Lo script copia i file, riscrive `SOURCE` e lancia `go test ./webui/ -run 'TestAssets|TestAPISurface'`.
`TestAssetsMatchSource` fallisce se gli asset incorporati non corrispondono a `SOURCE`.

`TestAPISurfaceLock` legge l'`index.js` incorporato, ne estrae i parametri di query e i metodi HTTP che usa e
li confronta con gli insiemi attesi in `webui/surface_test.go` (`expectedQueries`, `expectedMethods`, ognuno
con chi risponde). Se il test fallisce, il messaggio elenca i nomi nuovi e quelli spariti: la pagina nuova
usa una parte di protocollo che il server non conosce. Per ogni nome nuovo:

1. guarda nella cronologia di superdufs a che cosa serve;
2. se è una funzione del server, implementala in `webui/` con il suo test; se è solo del browser (il server
   la ignora), aggiungi il nome all'insieme atteso con la nota;
3. per un nome sparito, toglilo dall'insieme atteso e, se non serve più, dal server;
4. rilancia `go test ./webui/`.

Non si pubblica un aggiornamento degli asset finché il test non passa: significherebbe offrire pulsanti che
il server non sa servire.

`TestOverlayNode` (richiede `node`, altrimenti è saltato) controlla con `node --check` che `index.js` e
`overlay.js` siano sintatticamente validi e lancia `webui/overlay/overlay_test.mjs`, che carica `index.js` e poi
l'overlay e verifica che la funzione sostituita si comporti come l'originale per i valori che l'overlay non
cambia. Se fallisce dopo un aggiornamento, un nome usato dall'overlay è cambiato in `index.js`: aggiorna
`overlay.js`.

Per controllare che il fork non tocchi file upstream oltre a quelli consentiti:

```bash
webui/scripts/check-footprint.sh   # stampa "footprint OK"
```

Il confronto è con `upstream/master` se esiste il remote `upstream`, altrimenti con `origin/master`;
`UPSTREAM_REF` sceglie un altro riferimento.

## Test

Dalla radice del repository (`RCLONE_CONFIG=/notfound` evita di leggere la configurazione locale):

```bash
go build ./...
go test ./webui/... ./cmd/serve/webdav/ ./lib/http/...
RCLONE_WEBUI=false go test ./cmd/serve/webdav/    # la suite upstream con l'interfaccia spenta
webui/scripts/check-footprint.sh
```

Il secondo giro di `cmd/serve/webdav` con `RCLONE_WEBUI=false` dimostra che a interfaccia spenta il
comportamento è quello upstream. Prima di consegnare conviene anche `go test -race` sugli stessi pacchetti
e `golangci-lint run ./webui/... ./cmd/serve/webdav/...`.

`webui/scripts/smoke.sh` è la prova su un binario vero: compila rclone in una cartella temporanea, serve
una cartella temporanea e verifica la pagina per i browser, l'elenco upstream per `curl`, le scritture con
`Origin`, `?json`, `?zip`, il rifiuto del `MOVE` su una cartella esistente, i token, i link pubblici e
l'assenza della pagina con `--webui=false`. Stampa `smoke OK` ed esce con `0`; richiede `curl` e `python3`.

`webui/faulty_test.go` prova l'interfaccia con un remote che fallisce o non risponde (`faultyFs`, un `fs.Fs`
che avvolge `local` e fa fallire o bloccare a comando elenchi, `NewObject` e `Put`).

Gli script si controllano con `bash -n webui/scripts/*.sh` e `shellcheck webui/scripts/*.sh`.
