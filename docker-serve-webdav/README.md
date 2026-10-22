# rclone serve webdav in Docker

Questa cartella avvia con Docker Compose l'immagine del fork, `ghcr.io/manprint/rclone`, in modalità
`rclone serve webdav`: un server WebDAV per qualunque remote di rclone (una cartella locale, S3, SFTP,
Google Drive, ...) che ai browser mostra l'interfaccia web di superdufs, con upload, download, ricerca,
modifica dei file di testo, link pubblici e un player audio e video che riprende la riproduzione.

| File | Contenuto |
|------|-----------|
| `docker-compose.yml` | Il servizio: immagine, utente, porta, volumi, healthcheck. Non contiene valori propri: legge tutto da `.env` |
| `.env.example` | Il modello di `.env`, con ogni variabile commentata e i default ottimizzati per elenchi veloci e streaming video |
| `rclone.conf.example` | Il modello di `rclone.conf` con remote di esempio (cartella locale, S3, SFTP, WebDAV, Google Drive, crypt, union) |
| `.gitignore` | Esclude da git `.env`, `config/`, `cache/`, `data/` e le altre cartelle locali, che contengono credenziali o dati |

Il comportamento completo dell'interfaccia (endpoint, link, sicurezza, limiti) è descritto in
[fork-doc/uso.md](../fork-doc/uso.md); qui c'è quello che serve per usarla con Docker.

## Indice

- [Avvio rapido](#avvio-rapido)
- [Come funziona la configurazione](#come-funziona-la-configurazione)
- [Variabili del compose](#variabili-del-compose)
- [Percorsi interni del container](#percorsi-interni-del-container)
- [Autenticazione e rete](#autenticazione-e-rete)
- [Elenchi veloci delle cartelle](#elenchi-veloci-delle-cartelle)
- [Cache e streaming video](#cache-e-streaming-video)
- [Flag nuovi dell'interfaccia web](#flag-nuovi-dellinterfaccia-web)
- [Flag di rclone che cambiano l'interfaccia](#flag-di-rclone-che-cambiano-linterfaccia)
- [Log](#log)
- [rclone.conf e remote](#rcloneconf-e-remote)
- [Reverse proxy e TLS](#reverse-proxy-e-tls)
- [Client WebDAV](#client-webdav)
- [Operazioni comuni](#operazioni-comuni)
- [Sicurezza](#sicurezza)
- [Corrispondenza con le opzioni di superdufs](#corrispondenza-con-le-opzioni-di-superdufs)
- [Problemi frequenti](#problemi-frequenti)

## Avvio rapido

Requisiti: Docker con il plugin Compose v2 (`docker compose version`). L'immagine è multi-architettura
(`linux/amd64`, `linux/arm64`).

```bash
cd docker-serve-webdav
cp .env.example .env
mkdir -p config cache data
```

Modifica almeno queste righe di `.env`:

```bash
PUID=1000              # id -u dell'utente proprietario di config/, cache/ e data/
PGID=1000              # id -g
RCLONE_USER=admin
RCLONE_PASS='una-password-robusta'
```

Le cartelle vanno create prima dell'avvio con l'utente `PUID`: se mancano Docker le crea di proprietà di
`root` e rclone, che non gira come root, non può scriverci.

```bash
docker compose up -d
docker compose ps            # STATUS deve diventare "healthy"
docker compose logs -f
```

Apri `http://<host>:8080/` con un browser: dopo le credenziali compare l'interfaccia con il contenuto di
`data/`. Lo stesso indirizzo funziona come server WebDAV per i client (vedi [Client WebDAV](#client-webdav)).

Per servire un remote invece della cartella locale copia il template della configurazione e scegli il
remote in `.env`:

```bash
cp rclone.conf.example config/rclone.conf   # e modificalo
# in .env: WEBDAV_REMOTE=s3:bucket/cartella
docker compose up -d
```

## Come funziona la configurazione

Tutti i valori stanno in un solo file, `.env`, e `docker-compose.yml` non ha default propri: se `.env`
manca o una variabile richiesta è vuota, `docker compose` si ferma con un messaggio come
`required variable WEBDAV_IMAGE is missing a value: imposta WEBDAV_IMAGE in .env`.

`.env` ha due usi:

1. Docker Compose ne legge le variabili `WEBDAV_*`, `PUID`, `PGID` e `COMPOSE_PROJECT_NAME` per costruire
   il servizio (immagine, porta, utente, volumi, remote servito).
2. Lo stesso file è passato al container come ambiente (`env_file`). rclone legge ogni variabile
   `RCLONE_<NOME>` come se fosse il flag `--<nome>`: il nome si ottiene togliendo `RCLONE_`, scrivendo in
   minuscolo e sostituendo `_` con `-`. `RCLONE_WEBUI_SEARCH_TIMEOUT=10s` equivale a
   `--webui-search-timeout 10s`, `RCLONE_VFS_CACHE_MODE=full` a `--vfs-cache-mode full`.

Vale per ogni flag di rclone, anche quelli non elencati in `.env.example`: l'elenco completo si ottiene con

```bash
docker compose run --rm --no-deps rclone-webdav serve webdav --help
docker compose run --rm --no-deps rclone-webdav help flags
```

Regole per i valori:

| Tipo | Formato | Esempi |
|------|---------|--------|
| Booleano | `true` / `false` | `RCLONE_WEBUI_RESUME=true` |
| Durata | numero con unità `ms`, `s`, `m`, `h`, `d`, `w`, `M`, `y` | `30s`, `1h`, `72h`, `3d` |
| Dimensione | numero con suffisso `K`, `M`, `G`, `T` (base 1024; `Mi` è lo stesso di `M`), `off` per nessun limite | `32M`, `4Mi`, `20G` |
| Lista | valori separati da virgola | `RCLONE_WEBUI_PUBLINK_PATHS=/a,/b` |

Attenzione a due dettagli:

- **Variabili vuote.** Una riga `RCLONE_X=` imposta comunque la variabile a stringa vuota: per booleani,
  durate, dimensioni ed enum rclone si ferma all'avvio con
  `CRITICAL: Invalid value when setting --x from environment variable RCLONE_X=""`. Le opzioni che non
  usi vanno lasciate commentate.
- **Il carattere `$`.** Docker Compose lo interpreta anche in `.env`. In password e segreti racchiudi il
  valore tra apici singoli (`RCLONE_PASS='pa$word'`: tra apici `$` resta `$`), oppure senza apici scrivi
  `$$` per ogni `$` (`RCLONE_PASS=pa$$word`). Senza queste precauzioni la password arriva a rclone
  diversa da come l'hai scritta e il login fallisce con `401`.

`.env` si legge solo all'avvio del container: dopo una modifica serve `docker compose up -d` (che ricrea
il container), non `docker compose restart`.

## Variabili del compose

Servono a Docker Compose e non sono flag di rclone.

| Variabile | Valore di `.env.example` | Descrizione |
|-----------|--------------------------|-------------|
| `COMPOSE_PROJECT_NAME` | `rclone-webdav` | Nome del progetto Compose (prefisso di rete e container) |
| `WEBDAV_IMAGE` | `ghcr.io/manprint/rclone:latest` | Immagine: `latest` è l'ultima release del fork, `X.Y.Z-webui.N` una release precisa (consigliata in produzione), `main` l'ultima build del ramo `main` |
| `WEBDAV_CONTAINER_NAME` | `rclone-webdav` | Nome del container |
| `WEBDAV_BIND` | `0.0.0.0` | Indirizzo dell'host su cui è pubblicata la porta; `127.0.0.1` la rende raggiungibile solo dalla macchina (reverse proxy) |
| `WEBDAV_PORT` | `8080` | Porta dell'host; nel container la porta è sempre `8080` |
| `PUID`, `PGID` | `1000` | Utente e gruppo con cui gira rclone. Devono poter scrivere in `config/` e `cache/` e leggere (o scrivere) `data/` |
| `WEBDAV_CONFIG_DIR` | `./config` | Cartella dell'host con `rclone.conf`, montata in `/config/rclone` |
| `WEBDAV_CACHE_DIR` | `./cache` | Cartella dell'host della cache VFS e delle posizioni di riproduzione, montata in `/cache` |
| `WEBDAV_DATA_DIR` | `./data` | Cartella dell'host montata in `/data` |
| `WEBDAV_REMOTE` | `/data` | Cosa servire: `/data` (la cartella `WEBDAV_DATA_DIR`), un remote (`s3:`), un percorso di un remote (`s3:bucket/cartella`) |

Le cartelle possono stare anche fuori dal repository (per esempio `WEBDAV_DATA_DIR=/srv/media`,
`WEBDAV_CACHE_DIR=/var/cache/rclone-webdav`): conviene per dati e cache grandi. Il build dell'immagine
del fork (`just image`) esclude comunque questa cartella dal contesto, ma un `docker build .` con il
`Dockerfile` upstream la includerebbe, con `.env` e la cache.

L'immagine da sola gira come `root` (contiene anche un utente `rclone`, uid/gid `1009`, che non usa per
default): il compose la fa girare come `PUID:PGID`, così rclone non ha privilegi e i file creati in
`data/` appartengono al tuo utente.

## Percorsi interni del container

Sono in `.env` (sezione 2) ma legati ai volumi e alla porta del compose: non vanno cambiati.

| Variabile | Valore | Flag | Motivo |
|-----------|--------|------|--------|
| `RCLONE_ADDR` | `:8080` | `--addr` | Porta interna, la stessa di `ports` e dell'healthcheck. Il default di rclone (`127.0.0.1:8080`) non sarebbe raggiungibile da fuori del container |
| `RCLONE_CONFIG` | `/config/rclone/rclone.conf` | `--config` | Il file di `WEBDAV_CONFIG_DIR` |
| `RCLONE_CACHE_DIR` | `/cache` | `--cache-dir` | La cache su `WEBDAV_CACHE_DIR`, persistente tra i riavvii |
| `RCLONE_WEBUI_RESUME_DIR` | `/cache/webui/resume` | `--webui-resume-dir` | Le posizioni di riproduzione, nel volume della cache |
| `TZ` | `Europe/Rome` | | Fuso orario del container (orari dei log) |

## Autenticazione e rete

Flag upstream di `rclone serve webdav`.

| Variabile | Flag | Default rclone | Descrizione |
|-----------|------|----------------|-------------|
| `RCLONE_USER` | `--user` | vuoto | Nome utente; senza utente né htpasswd il server non chiede credenziali |
| `RCLONE_PASS` | `--pass` | vuoto | Password di `RCLONE_USER` |
| `RCLONE_HTPASSWD` | `--htpasswd` | vuoto | File htpasswd con uno o più utenti; sostituisce `USER`/`PASS` |
| `RCLONE_REALM` | `--realm` | `rclone` | Realm dell'autenticazione Basic, usato anche dal logout dell'interfaccia |
| `RCLONE_USER_FROM_HEADER` | `--user-from-header` | vuoto | Utente letto da un header impostato dal reverse proxy (il proxy fa il login) |
| `RCLONE_SALT` | `--salt` | | Salt per l'hash delle password usato con `--auth-proxy` |
| `RCLONE_AUTH_PROXY` | `--auth-proxy` | vuoto | Programma che crea il backend in base alle credenziali; spegne token di download e link pubblici |
| `RCLONE_BASEURL` | `--baseurl` | vuoto | Prefisso di tutti gli URL (`/webdav`), anche di asset e link pubblici |
| `RCLONE_CERT`, `RCLONE_KEY` | `--cert`, `--key` | vuoto | Certificato (con la catena) e chiave PEM per servire HTTPS direttamente |
| `RCLONE_CLIENT_CA` | `--client-ca` | vuoto | CA dei certificati client (mTLS) |
| `RCLONE_ALLOW_ORIGIN` | `--allow-origin` | vuoto | Origine ammessa per le richieste CORS |
| `RCLONE_READ_ONLY` | `--read-only` | `false` | Sola lettura per tutti |
| `RCLONE_DISABLE_ZIP` | `--disable-zip` | `false` | Spegne lo scarico delle cartelle come zip |
| `RCLONE_EXCLUDE` | `--exclude` | vuoto | Pattern di file e cartelle nascosti e non accessibili, separati da virgola, senza graffe: `*.log,*.lock,tmp/**` |
| `RCLONE_MAX_HEADER_BYTES` | `--max-header-bytes` | `4096` | Dimensione massima degli header di una richiesta |
| `RCLONE_ETAG_HASH` | `--etag-hash` | vuoto | Hash usato come ETag (`auto`, `md5`, ...) |
| `RCLONE_SERVER_READ_TIMEOUT` | `--server-read-timeout` | `1h` | Durata massima di una richiesta, corpo compreso. `.env.example`: `12h` |
| `RCLONE_SERVER_WRITE_TIMEOUT` | `--server-write-timeout` | `1h` | Durata massima di una risposta. `.env.example`: `12h` |

I due timeout sono portati a `12h` perché una risposta dura quanto il trasferimento: un video guardato per
intero in una sola richiesta, il download di un file o di uno zip grande, l'upload su una linea lenta
supererebbero l'ora e sarebbero troncati.

### Utente singolo o htpasswd

`RCLONE_USER`/`RCLONE_PASS` vanno bene per provare, ma la password è leggibile da chiunque possa eseguire
`docker inspect` sul container. Per un uso reale crea un file htpasswd (bcrypt), montalo e usa
`RCLONE_HTPASSWD`:

```bash
mkdir -p auth
docker run --rm httpd:2.4-alpine htpasswd -nbB alice 'password-di-alice' >> auth/htpasswd
docker run --rm httpd:2.4-alpine htpasswd -nbB bob   'password-di-bob'   >> auth/htpasswd
```

In `docker-compose.yml` togli il commento a `- ./auth:/auth:ro` e in `.env`:

```bash
# RCLONE_USER=admin
# RCLONE_PASS=...
RCLONE_HTPASSWD=/auth/htpasswd
```

Tutti gli utenti di uno stesso server vedono gli stessi file con gli stessi permessi: rclone non ha
permessi per utente o per cartella. Per separarli si usano più container (un remote o un percorso
ciascuno, eventualmente `RCLONE_READ_ONLY=true`) oppure `RCLONE_AUTH_PROXY`.

## Elenchi veloci delle cartelle

Il server vede il remote attraverso la VFS di rclone, che tiene in memoria gli elenchi delle cartelle.
Con i default di rclone ogni cartella è riletta dal remote dopo 5 minuti, e su un remote lento o a
pagamento per richiesta questo si sente a ogni apertura della pagina e in ogni ricerca. `.env.example`
imposta invece:

| Variabile | Flag | Default rclone | `.env.example` | Effetto |
|-----------|------|----------------|----------------|---------|
| `RCLONE_DIR_CACHE_TIME` | `--dir-cache-time` | `5m` | `1h` | Per quanto un elenco resta valido prima di essere riletto |
| `RCLONE_POLL_INTERVAL` | `--poll-interval` | `1m` | `1m` | Ogni quanto si chiedono al remote i cambiamenti; deve essere minore di `RCLONE_DIR_CACHE_TIME`; `0` lo spegne |
| `RCLONE_VFS_REFRESH` | `--vfs-refresh` | `false` | `true` | All'avvio legge in background l'intero albero, così la prima apertura di ogni cartella e la ricerca non aspettano il remote |
| `RCLONE_VFS_FAST_FINGERPRINT` | `--vfs-fast-fingerprint` | `false` | `true` | Riconosce i file in cache da dimensione e data senza calcolare hash: più veloce su S3, Swift e simili |

Cosa aspettarsi:

- Le modifiche fatte attraverso questo server (interfaccia o client WebDAV) aggiornano subito la cache.
- Le modifiche fatte da fuori arrivano con il polling sui remote che lo supportano (Google Drive,
  OneDrive, Dropbox, Box, pCloud, ...) e, sugli altri, alla scadenza di `RCLONE_DIR_CACHE_TIME`.
  La cartella locale (`/data`) non supporta il polling: se modifichi spesso i file direttamente sull'host,
  porta `RCLONE_DIR_CACHE_TIME` a `1m`.
- `RCLONE_VFS_REFRESH` usa l'elenco ricorsivo del backend (ListR) quando c'è (S3, B2, Swift, Google
  Drive, ...): poche richieste anche per alberi grandi. Su remote enormi (milioni di file) o senza ListR
  costa una richiesta per cartella e molta memoria: in quel caso mettilo a `false`.
- `--fast-list` non serve: `serve webdav` elenca una cartella alla volta, e il refresh usa ListR da solo.
- La ricerca dell'interfaccia (`--webui-search`) e il conteggio degli elementi (`--webui-dir-count`)
  leggono gli elenchi dalla stessa cache: con la cache calda sono immediati.

## Cache e streaming video

| Variabile | Flag | Default rclone | `.env.example` | Effetto |
|-----------|------|----------------|----------------|---------|
| `RCLONE_VFS_CACHE_MODE` | `--vfs-cache-mode` | `off` | `full` | Cache su disco dei file letti e scritti |
| `RCLONE_VFS_CACHE_MAX_SIZE` | `--vfs-cache-max-size` | `off` | `20G` | Dimensione massima della cache |
| `RCLONE_VFS_CACHE_MAX_AGE` | `--vfs-cache-max-age` | `1h` | `24h` | I file non letti da più di questo tempo escono dalla cache |
| `RCLONE_VFS_CACHE_MIN_FREE_SPACE` | `--vfs-cache-min-free-space` | `off` | `2G` | Spazio minimo da lasciare libero sul disco della cache |
| `RCLONE_VFS_READ_CHUNK_SIZE` | `--vfs-read-chunk-size` | `128M` | `32M` | Dimensione del primo blocco letto dal remote |
| `RCLONE_VFS_READ_CHUNK_SIZE_LIMIT` | `--vfs-read-chunk-size-limit` | `off` | `1G` | Limite del raddoppio dei blocchi successivi |
| `RCLONE_VFS_READ_CHUNK_STREAMS` | `--vfs-read-chunk-streams` | `0` | `0` | Letture parallele a blocchi fissi (vedi sotto) |
| `RCLONE_VFS_READ_AHEAD` | `--vfs-read-ahead` | `0` | `256M` | Lettura anticipata su disco oltre al buffer (solo con `full`) |
| `RCLONE_BUFFER_SIZE` | `--buffer-size` | `16M` | `32M` | Buffer in memoria per ogni file aperto |
| `RCLONE_VFS_HANDLE_CACHING` | `--vfs-handle-caching` | `5s` | `30s` | Tiene vivo il lettore del remote dopo la chiusura del file |
| `RCLONE_VFS_WRITE_BACK` | `--vfs-write-back` | `5s` | (default) | Ritardo dell'upload verso il remote dopo la chiusura di un file |
| `RCLONE_TRANSFERS` | `--transfers` | `4` | (default) | Upload paralleli dalla cache verso il remote |
| `RCLONE_VFS_DISK_SPACE_TOTAL_SIZE` | `--vfs-disk-space-total-size` | `off` | (commentata) | Totale fisso per le statistiche di spazio |
| `RCLONE_VFS_USED_IS_SIZE` | `--vfs-used-is-size` | `false` | (commentata) | Spazio usato calcolato sommando i file (visita tutto il remote) |
| `RCLONE_NO_MODTIME` | `--no-modtime` | `false` | (commentata) | Non legge le date di modifica: più veloce su S3 e Swift, ma la colonna data resta vuota |

Perché questi valori:

- **`full`.** Un browser legge un video con richieste `Range` e, a ogni seek, salta in un altro punto del
  file. Con `full` le parti già lette restano su disco e un seek indietro non riscarica nulla; con `off`
  ogni richiesta torna al remote. `full` (o `writes`) serve anche a riprendere gli upload interrotti
  dall'interfaccia (`PATCH` append): con `off` o `minimal` un upload interrotto si ricomincia da capo.
- **Blocchi.** Un primo blocco di `32M` invece di `128M` fa partire prima il video e ogni seek; i blocchi
  successivi raddoppiano (`64M`, `128M`, ...) fino a `1G`, così la riproduzione continua chiede pochi
  blocchi grandi.
- **Read-ahead.** Con `full` rclone legge `256M` in anticipo su disco: il player non resta senza dati se
  il remote rallenta per qualche secondo.
- **Handle caching.** Tra una richiesta `Range` e la successiva il lettore aperto sul remote resta vivo
  `30s` invece di `5s`, così la richiesta dopo non deve riaprirlo.
- **Letture parallele.** Su remote ad alta latenza (S3, Swift, storage lontani) un solo flusso può non
  bastare per video ad alto bitrate: prova `RCLONE_VFS_READ_CHUNK_STREAMS=16` con
  `RCLONE_VFS_READ_CHUNK_SIZE=4M` (in questa modalità i blocchi non raddoppiano). Sulla cartella locale
  non serve.

Dimensiona `WEBDAV_CACHE_DIR` su un disco con almeno `RCLONE_VFS_CACHE_MAX_SIZE` +
`RCLONE_VFS_CACHE_MIN_FREE_SPACE` liberi. Con `/data` locale la cache duplica i file letti: se servi solo
la cartella locale e il disco è stretto, `RCLONE_VFS_CACHE_MODE=writes` usa la cache solo per le scritture.

**La cache contiene anche gli upload non ancora arrivati sul remote.** Un file caricato è scritto prima in
`cache/` e copiato sul remote `RCLONE_VFS_WRITE_BACK` secondi dopo la chiusura. Se il container si ferma
prima, l'upload resta in `cache/` e riparte al riavvio successivo (nel log:
`vfs cache: queuing for upload`). Per questo `cache/` deve essere persistente e non va cancellata mentre
il log riporta upload in sospeso (`to upload N` con `N` maggiore di zero).

## Flag nuovi dell'interfaccia web

Sono i flag aggiunti dal fork. Ognuno ha la sua variabile `RCLONE_WEBUI_*` e appare in
`rclone serve webdav --help`. Nell'help `--webui-token-expiry` e `--webui-resume-retention` mostrano i
default come `3d` e `1M`, che equivalgono a `72h` e `720h`.

| Variabile | Flag | Default | `.env.example` | Descrizione |
|-----------|------|---------|----------------|-------------|
| `RCLONE_WEBUI` | `--webui` | `true` | `true` | Serve l'interfaccia ai browser che aprono una cartella; `false` = server identico a upstream |
| `RCLONE_WEBUI_ALLOW_DELETE` | `--webui-allow-delete` | `true` | (default) | `false` = upload senza cancellazione: nessun client può cancellare, spostare o sovrascrivere (vedi sotto) |
| `RCLONE_WEBUI_SECRET` | `--webui-secret` | vuoto | (commentata) | Segreto che firma token di download e link pubblici. Vuoto = chiavi casuali, i link smettono di funzionare a ogni riavvio |
| `RCLONE_WEBUI_TOKEN_EXPIRY` | `--webui-token-expiry` | `72h` | (default) | Durata dei token di download; `<= 0` li fa scadere subito |
| `RCLONE_WEBUI_PUBLINK_PATHS` | `--webui-publink-paths` | vuoto | (commentata) | Percorsi, separati da virgola, per cui la pagina offre link pubblici permanenti; `/` = tutto il server |
| `RCLONE_WEBUI_SEARCH` | `--webui-search` | `true` | (default) | Ricerca ricorsiva per nome |
| `RCLONE_WEBUI_SEARCH_MAX_DEPTH` | `--webui-search-max-depth` | `8` | (default) | Livelli di sottocartelle esplorati; `0` = solo la cartella corrente |
| `RCLONE_WEBUI_SEARCH_MAX_RESULTS` | `--webui-search-max-results` | `1000` | (default) | Risultati massimi |
| `RCLONE_WEBUI_SEARCH_TIMEOUT` | `--webui-search-timeout` | `30s` | (default) | Durata massima di una ricerca; allo scadere restituisce i risultati trovati fin lì |
| `RCLONE_WEBUI_RESUME` | `--webui-resume` | `false` | `true` | Ricorda la posizione di riproduzione di audio e video |
| `RCLONE_WEBUI_RESUME_DIR` | `--webui-resume-dir` | `<cache-dir>/webui/resume` | `/cache/webui/resume` | Cartella delle posizioni (fissa, vedi [Percorsi interni](#percorsi-interni-del-container)) |
| `RCLONE_WEBUI_RESUME_RETENTION` | `--webui-resume-retention` | `720h` | (default) | Le posizioni non aggiornate da più di questo tempo sono cancellate; `0` = per sempre |
| `RCLONE_WEBUI_PLAYER_MAX_RETRY` | `--webui-player-max-retry` | `30` | (default) | Tentativi di riconnessione del player quando il flusso si interrompe |
| `RCLONE_WEBUI_PLAYER_RECONNECT_INTERVAL` | `--webui-player-reconnect-interval` | `10` | (default) | Secondi tra due tentativi di riconnessione |
| `RCLONE_WEBUI_DISK_STATS` | `--webui-disk-stats` | `false` | (commentata) | Mostra spazio usato e libero (dal metodo `About` del backend) |
| `RCLONE_WEBUI_UPLOAD_SETTINGS` | `--webui-upload-settings` | `false` | (commentata) | Mostra le impostazioni degli upload concorrenti del browser |
| `RCLONE_WEBUI_DIR_COUNT` | `--webui-dir-count` | `false` | (commentata) | Mostra il numero di elementi delle cartelle; costa un elenco per sottocartella a ogni apertura |
| `RCLONE_WEBUI_EDIT_MAX_SIZE` | `--webui-edit-max-size` | `4Mi` | (default) | Dimensione massima di un file di testo modificabile nella pagina |
| `RCLONE_WEBUI_MULTI_ZIP_MAX` | `--webui-multi-zip-max` | `200` | (default) | Elementi massimi di uno zip da selezione multipla |

"(default)" indica una variabile presente commentata in `.env.example` con il valore di default;
"(commentata)" una funzione spenta che si accende togliendo il commento.

### Upload senza cancellazione

Con `RCLONE_WEBUI_ALLOW_DELETE=false` gli utenti possono caricare file e creare cartelle, ma non cancellare,
spostare, rinominare o sovrascrivere quello che c'è già: la pagina nasconde quei comandi e il server li
rifiuta con `403` a **tutti** i client, browser e client WebDAV. Un file nuovo si può creare e un file vuoto
riempire una volta (come "New File" della pagina); un `PUT` su un file non vuoto, `DELETE`, `MOVE` e `COPY`
su una destinazione esistente sono rifiutati. L'esistenza del file è verificata anche sul backend, quindi
la protezione vale anche per i file aggiunti da fuori e non ancora nella cache degli elenchi.

Va bene per una cartella di consegna o un archivio in cui si deposita soltanto. Non va bene con programmi
che salvano sovrascrivendo o con file temporanei rinominati (editor e suite d'ufficio su un disco WebDAV
montato, `rclone sync`): ricevono `403`. Il controllo è dell'interfaccia: con `RCLONE_WEBUI=false` è
ignorato e il log lo segnala. Dettagli in [fork-doc/uso.md](../fork-doc/uso.md#upload-senza-cancellazione).

Il flag c'è dalla release `1.75.2-webui.3` (e in `main`): un'immagine precedente ignora la variabile
senza errori. Per controllare:
`docker compose run --rm --no-deps rclone-webdav serve webdav --help | grep webui-allow-delete`.

### Link di download e link pubblici

Con l'autenticazione attiva la pagina crea per ogni file un link scaricabile senza credenziali
(`?token=...`, valido `RCLONE_WEBUI_TOKEN_EXPIRY`). I link pubblici (`/@public/<token>/<percorso>`) sono
permanenti e di sola lettura e si attivano elencando i percorsi:

```bash
RCLONE_WEBUI_SECRET='frase-lunga-casuale'        # openssl rand -base64 32
RCLONE_WEBUI_PUBLINK_PATHS=/condivisi,/foto/2026
```

Imposta sempre `RCLONE_WEBUI_SECRET` quando usi i link: senza, sono firmati con chiavi casuali e muoiono a
ogni riavvio del container. Un link pubblico si revoca togliendo il percorso da
`RCLONE_WEBUI_PUBLINK_PATHS` o cambiando il segreto (che invalida tutti i link). Server diversi devono
avere segreti diversi.

### Ripresa della riproduzione

Con `RCLONE_WEBUI_RESUME=true` il player salva la posizione di `mp4`, `mov`, `avi`, `wmv`, `flv`, `webm`,
`mkv`, `mp3`, `ogg`, `wav`, `m4a` in `cache/webui/resume/` (un file JSON per file, permessi `0600`), e la
riprende alla visita successiva anche dopo un riavvio del container. Le posizioni sono condivise da tutti
gli utenti del server e non si scrive nulla sul remote.

## Flag di rclone che cambiano l'interfaccia

| Variabile | Effetto sulla pagina |
|-----------|----------------------|
| `RCLONE_READ_ONLY=true` | Nasconde upload e cancellazione (`allow_upload` e `allow_delete` falsi); le scritture sono rifiutate |
| `RCLONE_WEBUI_ALLOW_DELETE=false` | Nasconde cancellazione, spostamento e modifica (`allow_delete` falso); `?edit` di un file non vuoto è in sola lettura |
| `RCLONE_DISABLE_ZIP=true` | Nasconde lo scarico come zip; `?zip` risponde `403` |
| `RCLONE_BASEURL` | Prefisso di tutti gli URL della pagina, degli asset e dei link pubblici |
| `RCLONE_USER`/`RCLONE_PASS`/`RCLONE_HTPASSWD` | Attivano login, logout e token di download |
| `RCLONE_USER_FROM_HEADER` | Conta come autenticazione; token e link pubblici funzionano solo se il proxy lascia passare `?token=` e `/@public/` senza login |
| `RCLONE_AUTH_PROXY` | Spegne token di download e link pubblici |
| `RCLONE_VFS_CACHE_MODE` | Con `off`/`minimal` gli upload interrotti non riprendono |
| `RCLONE_NO_MODTIME=true` | La colonna data resta vuota |
| `RCLONE_EXCLUDE` | I file esclusi non compaiono nell'elenco né nella ricerca e non si possono scaricare |
| `RCLONE_TEMPLATE`, `RCLONE_DISABLE_DIR_LIST=true` | Spengono l'interfaccia (comportamento upstream) |

## Log

| Variabile | Flag | `.env.example` | Descrizione |
|-----------|------|----------------|-------------|
| `RCLONE_LOG_LEVEL` | `--log-level` | `INFO` | `DEBUG`, `INFO`, `NOTICE`, `ERROR` |
| `RCLONE_LOG_FILE` | `--log-file` | (non usata) | Senza, il log va su stderr e si legge con `docker compose logs` |
| `RCLONE_LOG_FORMAT` | `--log-format` | (non usata) | Campi della riga di log (`date,time,microseconds,...`) |
| `RCLONE_USE_JSON_LOG` | `--use-json-log` | (non usata) | Log in JSON, per un collettore di log |

Il compose ruota il log di Docker (`json-file`, 3 file da 10 MB).

**Non usare `DEBUG` in produzione:** a quel livello rclone scrive nel log il valore di ogni variabile
d'ambiente che legge, comprese `RCLONE_PASS` e `RCLONE_WEBUI_SECRET`, in chiaro
(`Setting --pass "..." from environment variable RCLONE_PASS="..."`). Se l'hai usato per una prova, cambia
password e segreto o cancella i log del container.

## rclone.conf e remote

`rclone.conf.example` contiene un remote per tipo; copialo in `config/rclone.conf`, tieni quelli che ti
servono, sostituisci i valori tra `<>` e scegli il remote con `WEBDAV_REMOTE`:

| Remote del template | Tipo | Esempio di `WEBDAV_REMOTE` |
|---------------------|------|----------------------------|
| `locale` | `alias` di `/data` | `locale:` (uguale a `/data`, ma usabile dentro `crypt` e `union`) |
| `s3` | S3 e compatibili (AWS, MinIO, Wasabi, R2) | `s3:bucket/cartella` |
| `sftp` | SFTP | `sftp:/home/utente/dati` |
| `webdav` | Nextcloud, ownCloud, altro WebDAV | `webdav:` |
| `gdrive` | Google Drive | `gdrive:Film` |
| `cifrato` | `crypt` sopra `s3` | `cifrato:` |
| `unione` | `union` di `locale` e `s3` | `unione:` |

Le password in `rclone.conf` vanno offuscate:

```bash
docker run --rm ghcr.io/manprint/rclone:latest obscure 'la-password'
```

Un remote si può creare anche con la configurazione interattiva di rclone, che scrive in
`config/rclone.conf`:

```bash
docker compose run --rm --no-deps rclone-webdav config
```

Per Google Drive, OneDrive, Dropbox e gli altri remote OAuth il token si ottiene su una macchina con un
browser (`rclone authorize drive`) e si incolla nella configurazione. rclone lo rinnova e riscrive
`rclone.conf`: per questo `config/` è montata in scrittura e deve appartenere a `PUID`.

Le credenziali possono anche stare in `.env` invece che in `rclone.conf`, con
`RCLONE_CONFIG_<REMOTE>_<OPZIONE>` (nome del remote e dell'opzione in maiuscolo):

```bash
RCLONE_CONFIG_S3_ACCESS_KEY_ID=AKIA...
RCLONE_CONFIG_S3_SECRET_ACCESS_KEY='...'
```

Un remote si può definire anche solo così, senza `rclone.conf`: `RCLONE_CONFIG_MIOS3_TYPE=s3` più le sue
opzioni, e `WEBDAV_REMOTE=mios3:bucket`.

Per servire una cartella locale diversa da `./data` cambia `WEBDAV_DATA_DIR` (per esempio
`/srv/media`); per un file di chiavi SSH o certificati aggiungi un volume in sola lettura nel compose.

## Reverse proxy e TLS

Il modo consigliato per esporre il server su Internet è dietro un reverse proxy con HTTPS, con la porta
pubblicata solo in locale:

```bash
WEBDAV_BIND=127.0.0.1
WEBDAV_PORT=8080
# per servire sotto un percorso, es. https://example.com/webdav/
# RCLONE_BASEURL=/webdav
```

Caddy:

```
files.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

nginx (upload grandi e streaming senza buffer):

```nginx
server {
    listen 443 ssl;
    server_name files.example.com;
    # ssl_certificate ... ; ssl_certificate_key ... ;

    client_max_body_size 0;
    proxy_request_buffering off;
    proxy_buffering off;
    proxy_read_timeout 12h;
    proxy_send_timeout 12h;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Con `RCLONE_BASEURL=/webdav` la `location` diventa `/webdav/` e `proxy_pass` resta senza percorso, così il
prefisso arriva a rclone.

In alternativa rclone serve HTTPS da solo: metti certificato e chiave in `certs/`, togli il commento a
`- ./certs:/certs:ro` nel compose e imposta `RCLONE_CERT=/certs/fullchain.pem` e
`RCLONE_KEY=/certs/privkey.pem`.

## Client WebDAV

L'indirizzo è lo stesso del browser (`http(s)://host:porta/`, più `RCLONE_BASEURL` se impostato). I client
WebDAV e la riga di comando (`curl`, `wget`, rclone) ricevono le risposte di rclone upstream, senza
interfaccia.

| Client | Come |
|--------|------|
| rclone | `rclone config create casa webdav url=https://files.example.com vendor=rclone user=alice pass=$(rclone obscure '...')` |
| Linux (davfs2) | `sudo mount -t davfs https://files.example.com /mnt/files` |
| GNOME / KDE | `davs://files.example.com/` nel file manager |
| macOS Finder | Vai > Connessione al server > `https://files.example.com` |
| Windows | Connetti unità di rete > `https://files.example.com`; il client WebDAV di Windows richiede HTTPS per l'autenticazione Basic, salvo modifiche al registro |
| curl | `curl -u alice:... https://files.example.com/file.txt`, `curl -u alice:... -T file.txt https://files.example.com/` |

## Operazioni comuni

```bash
docker compose up -d                  # avvia, o applica le modifiche a .env e al compose
docker compose logs -f                # log
docker compose ps                     # stato e healthcheck
docker compose down                   # ferma e rimuove il container (config/, cache/, data/ restano)

docker compose pull && docker compose up -d     # aggiorna l'immagine
docker compose exec rclone-webdav rclone version

# comandi rclone con la stessa configurazione, senza fermare il server
docker compose run --rm --no-deps rclone-webdav listremotes
docker compose run --rm --no-deps rclone-webdav lsd s3:
docker compose run --rm --no-deps rclone-webdav about s3:
```

L'healthcheck verifica che la porta `8080` accetti connessioni (`nc -z`), senza credenziali.

Da salvare in un backup: `.env`, `config/` (configurazione e token), `auth/`, e `cache/webui/resume/` se
le posizioni di riproduzione contano. Il resto di `cache/` si ricrea, ma va lasciato in pace finché ci
sono upload in sospeso (vedi [Cache e streaming video](#cache-e-streaming-video)).

## Sicurezza

- **Credenziali nell'ambiente.** Ogni variabile di `.env` è leggibile con `docker inspect` da chi ha
  accesso al Docker dell'host. Preferisci `RCLONE_HTPASSWD` a `RCLONE_PASS`, e tieni `.env` con permessi
  `600` (`chmod 600 .env`). `.gitignore` lo esclude da git.
- **Log a livello DEBUG.** Scrivono password e segreto in chiaro: vedi [Log](#log).
- **Segreto dei link.** `RCLONE_WEBUI_SECRET` firma token di download e link pubblici: chi lo conosce può
  creare link validi per i percorsi di `RCLONE_WEBUI_PUBLINK_PATHS`. Usa una frase lunga e casuale e
  cambiala se trapela (tutti i link esistenti smettono di funzionare).
- **Senza autenticazione.** Se `RCLONE_USER` e `RCLONE_HTPASSWD` sono entrambe assenti chiunque raggiunga
  la porta può leggere, scrivere e cancellare. In quel caso pubblica la porta solo in locale
  (`WEBDAV_BIND=127.0.0.1`) o metti `RCLONE_READ_ONLY=true`.
- **HTTPS.** L'autenticazione Basic manda le credenziali a ogni richiesta: su reti non fidate esponi il
  server solo in HTTPS (reverse proxy o `RCLONE_CERT`/`RCLONE_KEY`).
- **Link pubblici.** Sono permanenti e senza credenziali; i loro token compaiono nei log delle richieste.
- **Utente non root.** Il container gira con `PUID:PGID`, non come root.

Il dettaglio delle protezioni dell'interfaccia (CSP, sandbox dei file HTML e SVG, CORS, controllo dei
percorsi) è in [fork-doc/uso.md](../fork-doc/uso.md#sicurezza).

## Corrispondenza con le opzioni di superdufs

L'interfaccia è quella di superdufs, con gli stessi file (`index.html`, `index.js`, `index.css`,
`favicon.ico` identici byte per byte) e gli stessi dati: il fork compila tutti i campi che la pagina
legge (`allow_upload`, `allow_delete`, `allow_search`, `allow_archive`, `auth`, `user`, `dir_exists`,
`playback_resume_enabled`, `publink_enabled`, `publink_is_viewer`, `enable_data_stats`,
`show_concurrent_uploads_settings`, `disk_usage`, `disk_total`, `disk_available`, `resume_position`,
`publink_token`, e per editor e player `editable`, `resilient_player_max_retry`,
`resilient_player_reconnect_interval`) e risponde a tutte le richieste che la pagina fa (`?q`, `?zip`,
`?zip=multi`, `?edit`, `?view`, `?disk-stats`, `?tokengen`, `?token`, `?resume-get`, `?resume-save`,
`?resume-clear`, `/@public/`, `CHECKAUTH`, `LOGOUT`, `PUT`, `PATCH` append, `MKCOL`, `MOVE`, `DELETE`).

Le opzioni di superdufs che governano la pagina corrispondono così:

| superdufs | Questo server | Note |
|-----------|---------------|------|
| `--allow-upload` | `RCLONE_READ_ONLY` (inverso) | `RCLONE_READ_ONLY=true` spegne upload e cancellazione insieme |
| `--allow-delete` | `RCLONE_WEBUI_ALLOW_DELETE` | Stesse regole di superdufs: senza, niente `DELETE` e `MOVE`, `PUT` solo su file nuovi o vuoti, `COPY` solo verso destinazioni nuove |
| `--allow-search` | `RCLONE_WEBUI_SEARCH` | Più i limiti `RCLONE_WEBUI_SEARCH_MAX_DEPTH`, `_MAX_RESULTS`, `_TIMEOUT` |
| `--allow-archive` | `RCLONE_DISABLE_ZIP` (inverso) | Più `RCLONE_WEBUI_MULTI_ZIP_MAX` |
| `--allow-all` | (default) | Tutto è permesso salvo `RCLONE_READ_ONLY`, `RCLONE_WEBUI_ALLOW_DELETE=false` e `RCLONE_DISABLE_ZIP` |
| `--auth user:pass@/dir:rw` | `RCLONE_USER`/`RCLONE_PASS`, `RCLONE_HTPASSWD` | **Senza ruoli per percorso**: tutti gli utenti hanno gli stessi permessi su tutto il server. Per separarli: più container, o `RCLONE_AUTH_PROXY` |
| `--publink-enabled <path>` | `RCLONE_WEBUI_PUBLINK_PATHS` | Stessa semantica: elenco di percorsi, `/` = tutto |
| `--publink-secret` | `RCLONE_WEBUI_SECRET` | Firma anche i token di download |
| `--enable-data-stats` | `RCLONE_WEBUI_DISK_STATS` | |
| `--show-concurrent-uploads-settings` | `RCLONE_WEBUI_UPLOAD_SETTINGS` | |
| `--playback-resume-enabled`, `-dir`, `-retention` | `RCLONE_WEBUI_RESUME`, `_DIR`, `_RETENTION` | |
| `--resilient-player-max-retry`, `-reconnect-interval` | `RCLONE_WEBUI_PLAYER_MAX_RETRY`, `_RECONNECT_INTERVAL` | |
| `--hidden tmp,*.log` | `RCLONE_EXCLUDE=tmp/**,*.log` | Filtri di rclone: i file esclusi non sono nemmeno scaricabili |
| `--path-prefix` | `RCLONE_BASEURL` | |
| `--enable-cors` | `RCLONE_ALLOW_ORIGIN` | rclone vuole l'origine esplicita (`*` per tutte) |
| `--tls-cert`, `--tls-key` | `RCLONE_CERT`, `RCLONE_KEY` | |
| `--bind`, `--port` | `WEBDAV_BIND`, `WEBDAV_PORT` | |
| `serve-path` | `WEBDAV_REMOTE` | Qualunque remote di rclone, non solo una cartella |
| `--log-file`, `--log-format` | `RCLONE_LOG_FILE`, `RCLONE_LOG_FORMAT` | Formato diverso: rclone non ha il formato dei log HTTP di dufs |
| `--allow-symlink` | (non applicabile) | La VFS di rclone non espone i symlink |
| `--allow-hash` (`?hash`) | **non disponibile** | La pagina non lo usa |
| `--assets` | **non disponibile** | Gli asset sono incorporati nel binario; `RCLONE_TEMPLATE` sostituisce la pagina ma spegne l'interfaccia |
| `--render-index`, `--render-try-index`, `--render-spa` | **non disponibili** | Servire `index.html` al posto dell'elenco non è previsto |
| `--compress` | **non disponibile** | Livello di compressione degli zip |
| (fisso in superdufs) | `RCLONE_WEBUI_TOKEN_EXPIRY`, `RCLONE_WEBUI_EDIT_MAX_SIZE`, `RCLONE_WEBUI_DIR_COUNT` | Opzioni in più del fork |

Le opzioni di superdufs che non riguardano la pagina ma il suo server (`--s3-*`, `--http-dav-config`,
`--stream-chunk-size`, `--rclone-*`, `--cleaner-*`, `--paths-watched-for-availability*`) non hanno
corrispondenti: qui il backend è direttamente rclone, e lo streaming si regola con le variabili di
[Cache e streaming video](#cache-e-streaming-video).

## Problemi frequenti

| Sintomo | Causa e soluzione |
|---------|-------------------|
| `required variable ... is missing a value` | Manca `.env` o la variabile indicata: `cp .env.example .env` |
| Il container si riavvia, nel log `CRITICAL: Invalid value when setting --x from environment variable RCLONE_X=""` | Una variabile è presente ma vuota: commentala o dalle un valore |
| `permission denied` su `/config`, `/cache` o `/data` | Le cartelle dell'host non appartengono a `PUID:PGID` (spesso perché le ha create Docker come root): `sudo chown -R $(id -u):$(id -g) config cache data` |
| `401` con la password giusta | La password contiene `$`: racchiudila tra apici singoli in `.env`, poi `docker compose up -d` |
| Il browser mostra l'elenco semplice di rclone invece dell'interfaccia | `RCLONE_WEBUI=false`, `RCLONE_TEMPLATE` o `RCLONE_DISABLE_DIR_LIST` impostate; oppure la richiesta non viene da un browser (`curl`, `wget` ricevono sempre l'elenco upstream) |
| File cambiati sull'host non compaiono | Cache degli elenchi: compaiono entro `RCLONE_DIR_CACHE_TIME`; abbassala (es. `1m`) se succede spesso |
| Un file caricato non c'è ancora sul remote | È in `cache/` in attesa dell'upload (`RCLONE_VFS_WRITE_BACK`); se il container è stato fermato riparte al riavvio |
| I link pubblici o di download smettono di funzionare dopo un riavvio | Manca `RCLONE_WEBUI_SECRET` |
| `403 Forbidden` cancellando, rinominando o salvando un file esistente | `RCLONE_WEBUI_ALLOW_DELETE=false`: è il comportamento voluto; rimettilo a `true` per permetterlo |
| La ripresa degli upload non funziona | `RCLONE_VFS_CACHE_MODE` è `off` o `minimal`: usa `writes` o `full` |
| Video che va a scatti da un remote lontano | Aumenta `RCLONE_VFS_READ_AHEAD`, oppure prova `RCLONE_VFS_READ_CHUNK_STREAMS=16` con `RCLONE_VFS_READ_CHUNK_SIZE=4M` |
| Una modifica a `.env` non ha effetto | `.env` si legge alla creazione del container: `docker compose up -d`, non `restart` |
| Avvio lento o molte richieste al remote all'avvio | `RCLONE_VFS_REFRESH=true` su un remote molto grande: mettilo a `false` |
