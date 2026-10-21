# webui — interfaccia web per `rclone serve webdav`

Questo package aggiunge a `rclone serve webdav` l'interfaccia web di
superdufs (un fork di [dufs](https://github.com/sigoden/dufs)): un browser che apre il server vede una pagina
con navigazione delle cartelle, upload, download e gestione dei file invece del semplice elenco HTML.
I client WebDAV e la riga di comando non sono interessati: il protocollo WebDAV resta quello upstream.

Il package è un'aggiunta del fork: l'unica modifica a file upstream sono tre righe marcate
`// fork:webui` in `cmd/serve/webdav/webdav.go`; il resto è in `webui/` e in
`cmd/serve/webdav/webdav_webui*.go`. Lo script `scripts/check-footprint.sh` lo verifica.

## Flag

L'interfaccia è attiva per default. Resta spenta (comportamento identico a upstream) con `--webui=false`,
con `--template` oppure con `--disable-dir-list`. Ogni flag ha la corrispondente variabile d'ambiente
`RCLONE_<NOME>` (es. `RCLONE_WEBUI_SEARCH`) e, per `rc`, il blocco di opzioni `webdav_webui`.

| Flag | Default | Descrizione |
|------|---------|-------------|
| `--webui` | `true` | Serve l'interfaccia web ai browser che aprono una cartella |
| `--webui-search` | `true` | Abilita la ricerca ricorsiva |
| `--webui-search-max-depth` | `8` | Profondità massima della ricerca |
| `--webui-search-max-results` | `1000` | Numero massimo di risultati della ricerca |
| `--webui-search-timeout` | `30s` | Durata massima di una ricerca |
| `--webui-publink-paths` | vuoto | Percorsi sotto i quali si possono creare link pubblici di download |
| `--webui-secret` | vuoto | Segreto con cui sono firmati i token dell'interfaccia (sensibile) |
| `--webui-token-expiry` | `72h` | Durata dei token di download |
| `--webui-resume` | `false` | Ricorda la posizione di riproduzione dei file multimediali |
| `--webui-resume-dir` | vuoto | Cartella dove sono salvate le posizioni di riproduzione |
| `--webui-resume-retention` | `720h` | Per quanto tempo si conservano le posizioni non usate |
| `--webui-player-max-retry` | `30` | Tentativi massimi di riconnessione del player |
| `--webui-player-reconnect-interval` | `10` | Secondi tra i tentativi di riconnessione del player |
| `--webui-disk-stats` | `false` | Mostra le statistiche di spazio disco |
| `--webui-upload-settings` | `false` | Mostra le impostazioni di upload concorrenti |
| `--webui-edit-max-size` | `4Mi` | Dimensione massima di un file modificabile dall'interfaccia |
| `--webui-dir-count` | `false` | Mostra il numero di elementi delle cartelle |
| `--webui-multi-zip-max` | `200` | Numero massimo di elementi di uno zip da selezione multipla |

Le funzioni associate a ciascun flag sono introdotte dalle fasi successive del piano: i flag sono già
registrati, ma finché la funzione non esiste il valore non ha effetto.

## Interfaccia web

Un browser che apre una cartella del server (`http://host:porta/`, `http://host:porta/cartella/`) riceve la
pagina superdufs con il contenuto reale della VFS: cartelle prima dei file, nomi in ordine naturale
(`a2` prima di `a10`, senza distinguere maiuscole), dimensione e data di modifica. L'ordinamento si cambia
con `?sort=name|mtime|size` e `?order=desc`; `order=desc` senza `sort` è ignorato e, quando c'è, inverte
l'intero elenco, cartelle comprese. La stessa lista in JSON si ottiene con `?json`, con qualsiasi client
e senza intestazione `Accept`. I dati incorporati nella pagina e quelli di `?json` sono identici.

Le pagine HTML escono con `Cache-Control: no-cache`, `X-Content-Type-Options: nosniff` e una
`Content-Security-Policy` che vieta frame di altri siti, `object` e `base` esterni. Gli asset (`index.js`,
`index.css`, `favicon.ico`, `overlay.js`) sono serviti sotto `/<baseurl>/__webui_<hash>__/` senza
autenticazione, con cache immutabile: l'hash cambia a ogni modifica degli asset.

### Fallback per la riga di comando

La pagina è servita solo se la richiesta è un `GET` o `HEAD` che accetta `text/html` (intestazione
`Accept`) **e** il suo `User-Agent` non inizia per `lynx/`, `w3m/`, `links `, `elinks/`, `curl/`, `wget/`,
`httpie/`, `aria2/`, `rclone/` (confronto senza distinzione di maiuscole). Tutte le altre richieste
passano al server WebDAV upstream: `curl`, `wget`, il backend `webdav` di rclone e i client WebDAV non
vedono differenze, con elenco HTML upstream, `PROPFIND`, `PUT` e il resto invariati. `?download=zip`
resta gestito dall'upstream. Le richieste che non sono `GET`/`HEAD` non sono toccate.

### Cartelle senza slash finale

Il browser che apre `/cartella` (senza `/` finale) è reindirizzato con `302` a `/cartella/`, mantenendo la
query e il prefisso di `--baseurl`: la pagina risolve i link relativi rispetto all'URL corrente e senza
slash puntavano alla cartella superiore. I client che non sono browser continuano a ricevere la risposta
upstream (`405` per un `GET` su una cartella senza slash). Se il percorso non esiste o è un file, la
richiesta passa all'upstream.

### Flag che influenzano la pagina

- `--read-only`: la pagina nasconde upload e cancellazione (`allow_upload` e `allow_delete` falsi).
- `--disable-zip`: la pagina non offre lo scarico come archivio (`allow_archive` falso).
- `--no-modtime`: le date di modifica sono `0` e la pagina lascia vuota la colonna.
- `--template`: l'interfaccia resta spenta e si ottiene il comportamento upstream con il template scelto.
- `--disable-dir-list`: l'interfaccia resta spenta.
- `--webui=false`: l'interfaccia resta spenta.
- `--webui-dir-count`: la colonna dimensione delle cartelle mostra il numero di elementi (al massimo 1000)
  invece di un trattino. Costa un elenco di ogni sottocartella a ogni apertura della pagina, quindi è
  spento per default; se una sottocartella non si può leggere resta il trattino.
- `--webui-search`, `--webui-disk-stats`, `--webui-upload-settings`: controllano soltanto i campi
  `allow_search`, `enable_data_stats` e `show_concurrent_uploads_settings` dei dati della pagina; le
  funzioni corrispondenti arrivano nelle fasi successive.

## Ripresa riproduzione

Con `--webui-resume` il player dell'interfaccia ricorda fino a dove è arrivata la riproduzione di un file
multimediale e riparte da lì alla visita successiva; l'elenco della cartella riporta la posizione
(`resume_position`, in secondi interi) accanto ai file che ne hanno una. Per default è spenta: senza il
flag le richieste di ripresa ricevono `404` e `playback_resume_enabled` è falso.

| Flag | Default | Effetto |
|------|---------|---------|
| `--webui-resume` | `false` | Attiva la memorizzazione delle posizioni |
| `--webui-resume-dir` | vuoto | Cartella delle posizioni; vuoto = `<cache dir di rclone>/webui/resume` |
| `--webui-resume-retention` | `720h` | Le posizioni non aggiornate da più di questo tempo sono cancellate; `0` le conserva sempre |

Le posizioni stanno solo sul disco locale del server, in file JSON (uno per file, `<sha256>.json`, permessi
`0600`) dentro la cartella indicata, creata con permessi `0700`. Non si scrive nulla sul remote, nemmeno con
`--read-only`. Se la cartella non si può creare il server scrive un errore nel log e parte lo stesso, con la
ripresa disattivata.

La posizione è per remote e percorso del file ed è condivisa da tutti gli utenti dello stesso remote: se
due utenti guardano lo stesso video, l'ultimo salvataggio vince. Rinominare o spostare un file ne perde la
posizione. La chiave dipende dal remote servito, quindi due server con remote diversi non si mescolano
anche se usano la stessa cartella.

Sono interessati i file con estensione (senza distinzione di maiuscole) `mp4`, `mov`, `avi`, `wmv`, `flv`,
`webm`, `mkv`, `mp3`, `ogg`, `wav`, `m4a`. L'interfaccia usa tre richieste `GET` (valide anche come `HEAD`)
su qualunque percorso:

- `?resume-get`: `200` con `{"position", "duration", "timestamp", "file_path"}`, `404` se non c'è una posizione.
- `?resume-save=<secondi>[&duration=<secondi>]`: `204`; `400` se la posizione non è un numero finito `>= 0`;
  `404` se il percorso non è un file multimediale esistente.
- `?resume-clear`: `204`, anche se non c'era una posizione.

Le risposte portano `Cache-Control: no-store`. Almeno una volta ogni 24 ore, al primo salvataggio, il server
cancella in background le posizioni più vecchie di `--webui-resume-retention`.

## Sincronizzazione

Gli asset (`assets/index.html`, `index.js`, `index.css`, `favicon.ico`) sono copiati byte per byte da
superdufs; `overlay/overlay.js` contiene le sole correzioni del fork e viene caricato dopo `index.js`.
`assets/SOURCE` registra repository, commit e sha256 degli asset, senza credenziali.

Per aggiornare gli asset da una copia locale di superdufs:

```bash
webui/scripts/sync-assets.sh /percorso/a/superdufs
```

Lo script copia i file, riscrive `SOURCE` e lancia `go test ./webui/ -run 'TestAssets|TestAPISurface'`.
Il test `TestAssetsMatchSource` fallisce se gli asset incorporati non corrispondono a `SOURCE`.

Per controllare che il fork non tocchi file upstream oltre a quelli consentiti:

```bash
webui/scripts/check-footprint.sh   # stampa "footprint OK"
```
