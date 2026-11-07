# `rclone serve webdav` su host piccoli (1 GB / 1 CPU)

Questa guida raccoglie le verifiche su memoria e CPU di `rclone serve webdav` del fork (interfaccia web compresa),
le correzioni fatte e i flag consigliati. Le correzioni al codice sono in tre backport:

- `webui/backports/08-http-progress-timeouts.patch` (`lib/http`, vale anche per `serve http`, `serve s3`,
  `serve restic`, `rcd`);
- `webui/backports/09-serve-webdav-small-hosts.patch` (`cmd/serve/webdav`);
- `webui/backports/11-vfs-failed-stream-uploads.patch` (`vfs`, vale per tutti i `serve` e per `mount`).

Tutti i numeri qui sotto sono misurati, con il metodo descritto in [Riprodurre le misure](#9-riprodurre-le-misure).

---

## 1. Cosa è stato fatto

| Problema | Effetto | Correzione |
|---|---|---|
| `--server-read-timeout` e `--server-write-timeout` (default `1h`) contavano dall'inizio della richiesta | ogni upload o download singolo più lungo di un'ora veniva troncato (per esempio 27 GB a 50 Mbit/s durano 72 minuti) | backport 08: i timeout limitano il tempo **senza progressi**, un trasferimento che muove dati non viene più interrotto |
| `PROPFIND` con `Depth: infinity` o senza `Depth` elenca tutto l'albero in una richiesta | 200.201 voci: 233 MiB di picco e 128 MB di risposta da una sola richiesta | backport 09: nuovo flag `--disable-depth-infinity` (403 `propfind-finite-depth`, RFC 4918); default invariato |
| MD5 calcolato due volte a ogni upload (`operations.Rcat` e backend `local`) | circa metà della CPU di un PUT | nessuna modifica al core: `--ignore-checksum` sul server (vedi [sezione 4](#4-cpu-degli-upload---ignore-checksum)) |
| Buffer di lettura di 16 MiB per trasferimento | 16 trasferimenti paralleli: 369 MiB | `--buffer-size 4M --use-mmap`: 80 MiB, stessa velocità |
| PUT interrotto a metà (client ucciso, connessione caduta) | il pezzo ricevuto restava salvato con il nome finale, come se fosse completo | backport 09: l'upload viene abbandonato e il file parziale cancellato (vedi [sezione 8](#8-backup-in-streaming-rclone-rcat)) |
| Upload fallito che sostituiva un file esistente | il vecchio file, già cancellato dal backend `local`, restava elencato fino alla scadenza della cache delle cartelle | backport 11: non viene più elencato |

Verificati e lasciati invariati: lo zip delle cartelle dell'interfaccia web (deflate costa poco anche su file già
compressi), la compressione gzip delle risposte (utile sui PROPFIND, costosa sui file di testo grandi: si spegne
dal client), l'elenco, la ricerca e l'upload dell'interfaccia web (vedi [sezione 6](#6-interfaccia-web-e-compressione)).

### Test aggiunti

- `lib/http/server_timeout_test.go`, su HTTP/1.1 e h2c, con timeout di 500 ms:
  - `TestServerTimeoutSlowUpload`: upload lento ma continuo, 3 volte il timeout, completato;
  - `TestServerTimeoutSlowDownload`: download lento ma continuo, 3 volte il timeout, completato;
  - `TestServerTimeoutStalledUpload`: upload fermo, interrotto dal timeout.
- `cmd/serve/webdav/webdav_test.go`, `TestDisableDepthInfinity`: con il flag, `Depth` assente e `infinity`
  danno 403 con `<D:propfind-finite-depth/>`, `0` e `1` danno 207; senza il flag tutti danno 207.
- `cmd/serve/webdav/webdav_test.go`, `TestPutCutOff`: PUT con il corpo interrotto dopo 1 MiB, su un file nuovo
  e su uno esistente; il file nuovo non viene creato, quello esistente resta com'era.
- `vfs/write_test.go`:
  - `TestWriteFileHandleUploadFailed`: le scritture dopo il fallimento dell'upload, anche fuori ordine,
    restituiscono l'errore dell'upload;
  - `TestWriteFileHandleFailedReplace`: un file cancellato da un upload fallito che lo sostituiva non è più
    elencato, sia con l'upload fallito sia con quello abbandonato.

---

## 2. Timeout dei trasferimenti

`http.Server` di Go applica `ReadTimeout` e `WriteTimeout` dall'inizio di ogni richiesta: con il default di rclone
(`1h`) un PUT o un GET più lungo di un'ora veniva chiuso a metà, anche se i dati continuavano a passare. Il
backport 08 sposta in avanti le scadenze ogni volta che il server legge una parte del corpo della richiesta o
scrive una parte della risposta (al massimo una volta ogni `min(timeout/4, 1s)`). Leggere il corpo sposta anche la
scadenza di scrittura, perché la risposta a un upload arriva dopo la fine del corpo.

Misura con `--server-read-timeout 10s --server-write-timeout 10s`, 30 MiB a 1 MiB/s:

| | prima | dopo |
|---|---|---|
| PUT (`rclone copyto --bwlimit 1M`) | fallito a 11,1 s: `use of closed network connection` | 30,1 s, completato |
| GET (`curl --limit-rate 1M`) | troncato a 18,5 s: 19.565.280 byte su 31.457.280, curl rc=18 | 29,5 s, md5 identico |

Costo: nessuno misurabile. CPU del server per 2 GET da 1 GiB, tre giri alternati: prima 1469, 1207, 987 ms;
dopo 1253, 1051, 954 ms.

Un trasferimento fermo viene ancora chiuso dopo il timeout, quindi il default `1h` va bene anche per file
grandi. Con immagini del fork precedenti al backport i timeout restano sulla durata totale: per questo
`docker-serve-webdav/.env.example` li tiene a `12h`.

---

## 3. PROPFIND sull'intero albero e cache delle cartelle

Albero di prova: 200 cartelle da 1000 file vuoti (200.201 voci).

| Richiesta | Risultato | Tempo | Picco memoria (cgroup) |
|---|---|---|---|
| `PROPFIND /` senza `Depth` | 207, 133.789.947 byte (2.832.641 con gzip) | 5,0 s | 233 MiB, heap 166 MiB |
| idem con `--disable-depth-infinity` | 403, 99 byte | < 0,1 s | 20 MiB |
| `rclone lsf -R` (un `PROPFIND Depth: 1` per cartella) | 200.200 voci | 6,7 s | 237 MiB |

Il flag impedisce che **una sola richiesta** legga tutto l'albero, come `DavDepthInfinity Off` di Apache. Non
riduce la memoria di una visita completa legittima: la memoria è quella della cache delle cartelle della VFS,
circa 0,8 KB di heap per voce (157 MiB per 200.000 voci), qualunque sia il client che visita l'albero. Il client
`webdav` di rclone usa solo `Depth: 0` e `1` (`backend/webdav`); per gli altri client verificare che non chiedano
`Depth: infinity` prima di attivare il flag (nel log del server: `PROPFIND with Depth infinity ... rejected`).

La cache si libera dopo `2 × --dir-cache-time` (default 5m, quindi 10 minuti), poi serve un GC e Go restituisce
la memoria al sistema poco alla volta. Misura con `--dir-cache-time 10s` dopo `lsf -R`:

| Momento | RSS cgroup | Heap |
|---|---|---|
| fine della visita, fino a 90 s dopo | 235 MiB | 157 MiB |
| subito dopo un GC (`rclone rc core/gc`) | 232 MiB | 11 MiB |
| 30 s dopo il GC | 161 MiB | 11 MiB |
| 120 s dopo il GC | 29 MiB | 6 MiB |

Su un host da 1 GB con alberi da centinaia di migliaia di file: abbassare `--dir-cache-time` (per esempio `1m`
su un disco locale, dove rileggere una cartella costa poco) e impostare `GOMEMLIMIT`, che fa partire il GC prima
di arrivare al limite.

---

## 4. CPU degli upload: `--ignore-checksum`

Con `--vfs-cache-mode off` un PUT passa da `operations.Rcat`, che calcola l'MD5 del flusso, al backend `local`,
che lo calcola di nuovo sugli stessi byte. Nel profilo di un upload (sftp, stesso percorso) `crypto/md5.block` è
il 40% della CPU.

| PUT da 1 GiB (`rclone rcat --size`) | CPU del server |
|---|---|
| default | 4,4 - 4,7 s |
| `--ignore-checksum` sul server | 2,1 - 2,2 s |

Con un remote `local` il confronto fra i due MD5 non verifica niente: sono calcolati nello stesso processo sugli
stessi byte. `serve webdav` non espone hash ai client se non con `--etag-hash`, quindi i client non perdono
nessuna verifica. Con un remote vero dietro il server (S3, Drive, ...) `--ignore-checksum` toglie invece il
controllo fra rclone e il remote dopo l'upload.

`--etag-hash auto` su `local`: la prima volta che una cartella viene elencata (e di nuovo dopo la scadenza della
cache) il server legge per intero ogni file per calcolarne l'hash. Cartella con 365 MiB di file:
`PROPFIND Depth: 1` 0,7 s e 624 ms di CPU, contro 23 ms senza il flag; la seconda richiesta, con la cache
valida, 22 ms. Per ETag basati sull'hash su file grandi usare un remote `hasher`
(vedi [serve-s3.md](serve-s3.md)).

---

## 5. Memoria per trasferimento: `--buffer-size` e `--use-mmap`

Upload e download passano da un buffer di `--buffer-size` (default 16M) per trasferimento.

16 upload paralleli da 64 MiB, poi 16 download:

| Flag del server | Memoria anonima massima | Heap massimo | Upload | Download |
|---|---|---|---|---|
| default | 369 MiB | 367 MiB | 6,5 s | 1,5 s |
| `--buffer-size 4M` | 91 MiB | 87 MiB | 5,6 s | 1,1 s |
| `--buffer-size 4M --use-mmap` | 80 MiB | 15 MiB | 6,4 s | 1,2 s |

Un singolo trasferimento da 1 GiB, due giri per configurazione: PUT 4,5 - 4,7 s di CPU e GET 0,70 - 0,79 s in
tutte e tre le configurazioni. `--buffer-size 4M` non costa CPU con `serve webdav`.

---

## 6. Interfaccia web e compressione

### Elenco, ricerca e upload

| Operazione | Risultato | Tempo | CPU del server | Picco di memoria |
|---|---|---|---|---|
| elenco di una cartella con 20.000 file (`?json`) | 1.940.388 byte | 0,2 s | 257 ms | 64 MiB |
| ricerca su un albero di 200.201 voci, default | 200 risultati, albero visitato tutto | 1,7 s | 1706 ms | 196 MiB |
| stessa ricerca, `--webui-search-max-results 100` | 100 risultati | 0,9 s | 882 ms | 108 MiB |

La ricerca visita le cartelle in ampiezza attraverso la VFS, quindi la memoria è quella della cache delle
cartelle (vedi [sezione 3](#3-propfind-sullintero-albero-e-cache-delle-cartelle)) e si libera allo stesso modo
dopo `--dir-cache-time`. La visita si ferma al primo dei tre limiti: `--webui-search-max-results` (default
1000), `--webui-search-max-depth` (default 8) e `--webui-search-timeout` (default 30s). Una ricerca con pochi
risultati visita tutto l'albero fino a profondità e timeout: su alberi molto grandi e host da 1 GB conviene
ridurre `--webui-search-max-depth` o, se la ricerca non serve, `--webui-search=false`.

L'upload dall'interfaccia web (PUT con intestazione `Origin`) passa dallo stesso percorso di `operations.Rcat`
dei client WebDAV, con il doppio MD5. PUT da 1 GiB:

| | Interfaccia web | Client WebDAV |
|---|---|---|
| default | 4861 ms | 4461 ms |
| `--ignore-checksum` | 1903 ms | 1761 ms |

Memoria anonima massima 34 MiB in tutti e quattro i casi: il corpo della richiesta va sul disco senza essere
accumulato.

### Gzip e zip

Il server comprime con gzip le risposte `text/*` e XML (PROPFIND compresi) se il client manda
`Accept-Encoding: gzip`. Il client `webdav` di rclone lo manda sempre.

| GET di un log da 68.048.985 byte (`text/plain`) | Byte trasferiti | CPU del server |
|---|---|---|
| senza `Accept-Encoding` | 68.048.985 | 48 ms |
| con gzip (`curl --compressed`) | 12.964.334 | 456 ms |
| `rclone cat` | (gzip) | 444 ms |

Sui PROPFIND gzip conviene (133 MB diventano 2,8 MB). Sui file di testo grandi costa circa 10 volte la CPU:
in rete locale, con un server da 1 CPU, il client può rinunciarvi con `--no-gzip-encoding`.

Lo zip delle cartelle dell'interfaccia web (`?zip`) usa deflate anche per i file già compressi. Misura su 365 MiB
(3 file `.mp4` casuali da 100 MiB e un log da 65 MiB): 1,7 s, 1657 ms di CPU, picco 40 MiB, zip da
327.561.649 byte che passa `unzip -t`. Lo scarico semplice degli stessi file costa 1337 ms: deflate aggiunge il
24%, per questo lo zip è rimasto com'è.

---

## 7. Configurazione consigliata (host da 1 GB / 1 CPU)

Disco locale servito direttamente:

```bash
GOMEMLIMIT=600MiB rclone serve webdav /srv/data \
  --addr :8080 --htpasswd /etc/rclone/htpasswd \
  --buffer-size 4M --use-mmap \
  --ignore-checksum \
  --disable-depth-infinity \
  --dir-cache-time 1m
```

| Flag | Perché |
|---|---|
| `GOMEMLIMIT=600MiB` | il GC parte prima del limite del cgroup |
| `--buffer-size 4M --use-mmap` | 16 trasferimenti paralleli: 80 MiB invece di 369 MiB |
| `--ignore-checksum` | PUT con metà della CPU; su `local` non toglie verifiche |
| `--disable-depth-infinity` | una richiesta non può leggere tutto l'albero |
| `--dir-cache-time 1m` | la cache delle cartelle (0,8 KB per voce) si libera prima |
| `--vfs-cache-mode off` (default) | gli upload vanno direttamente sul disco, senza copia in cache |
| `--webui-search-max-depth` più basso o `--webui-search=false` | su alberi molto grandi la ricerca riempie la cache delle cartelle (vedi [sezione 6](#6-interfaccia-web-e-compressione)) |

Lato client rclone, in rete locale: `--no-gzip-encoding` per non far comprimere i file di testo al server.

Con `docker-serve-webdav` (remote veri, interfaccia web, `--vfs-cache-mode full` per seek dei video e ripresa
degli upload) valgono le stesse indicazioni per `--buffer-size`, `--use-mmap`, `--disable-depth-infinity` e
`GOMEMLIMIT`; `--ignore-checksum` lì toglie la verifica verso il remote.

---

## 8. Backup in streaming (`rclone rcat`)

Caso d'uso: da una VM con 3 GB liberi, `sudo tar -cpzf - mydir | rclone rcat webdav-served:dumps/mydir.tar.gz`
verso un server da 1 GB / 1 CPU, con un archivio da 27 GB. Né il client né il server devono tenere l'archivio
in RAM o in un file temporaneo.

### Verifica

Client e server in container `--memory=1g --cpus=1 --read-only`, con `/tmp` in un tmpfs da 64 MiB: qualunque
copia temporanea dello stream fallirebbe con `no space left on device`. Stream casuale da 1 GiB, md5 del file
sul server uguale a quello dello stream:

| Server | Tempo | CPU server | Memoria anonima server | Memoria anonima client | CPU client (rclone) |
|---|---|---|---|---|---|
| default | 22 s | 7,4 s | 27 MiB | 21 MiB | 3,3 s |
| `--ignore-checksum` | 21 s | 3,5 s | 24 MiB | 21 MiB | 3,0 s |

- `/tmp` resta vuoto su client e server; sul disco del server c'è un solo file, grande quanto la parte ricevuta.
- La memoria, misurata fino a 1 GiB, è piatta: con 256 MiB era 23 MiB sul server e 17 MiB sul client. Su
  entrambi i lati i dati passano per buffer di dimensione fissa, quindi per l'archivio da 27 GB ci si aspetta
  lo stesso; sul disco del server serve solo lo spazio del file.
- Il tempo è limitato dalla CPU del container client, condivisa con la generazione dei dati e l'md5 di
  controllo.
- Lo stream da 27 GB dura più di un'ora a 50 Mbit/s: senza il backport 08 veniva troncato (vedi
  [sezione 2](#2-timeout-dei-trasferimenti)).

### Requisiti

- **Client: `vendor = rclone`.** Il backend webdav carica uno stream di dimensione sconosciuta solo con i
  vendor `rclone`, `owncloud` e `fastmail`; con gli altri (`other`, `nextcloud`, ...) `rcat` copia prima
  tutto lo stream in `$TMPDIR`, e con 3 GB liberi fallisce con
  `failed to copy to temporary spool file: ... no space left on device`. Il messaggio
  `no hashes in common; falling back to --size-only` è normale.
- **Server: `--vfs-cache-mode off`** (default). Con `writes` o `full` il PUT viene scritto prima nella cache e
  poi copiato nella destinazione: serve il doppio dello spazio, e con la cache piena il client riceve
  `405 Method Not Allowed`. Il README di `docker-serve-webdav` propone `full` per l'interfaccia web: per i
  backup serve un'istanza separata con la cache spenta.

### Errori e interruzioni

| Caso | Cosa succede |
|---|---|
| client ucciso o connessione caduta a metà | il file parziale viene cancellato (backport 09; prima restava salvato come completo) |
| disco del server pieno | il client riceve `405 Method Not Allowed` e il file parziale viene cancellato; la causa (`no space left on device`) è solo nel log del server, perché il gestore WebDAV risponde sempre 405 quando la scrittura fallisce |
| `tar` fallisce a metà | rclone vede solo la fine dello stream e salva l'archivio troncato come completo: lo rivela soltanto il codice di uscita della pipeline con `set -o pipefail` |

- Uno stream non viene ritentato: rclone non può rileggere lo stdin, quindi dopo un errore il backup va rifatto.
- Il PUT scrive direttamente nel file finale: sovrascrivere un backup con lo stesso nome non richiede spazio
  in più, ma **se lo stream fallisce il vecchio backup è già perso**. Con 3 GB liberi e un archivio da 27 GB,
  usare nomi con la data e cancellare i vecchi backup solo dopo un backup riuscito.

Per non lasciare mai un archivio troncato con il nome definitivo, caricarlo con un nome temporaneo e rinominarlo
solo se tutta la pipeline è andata bene. Su WebDAV la rinomina è un `MOVE` sul server, istantaneo:

```bash
set -o pipefail
name=mydir-$(date +%F).tar.gz
if sudo tar -cpzf - mydir | rclone rcat webdav-served:dumps/$name.part; then
  rclone moveto webdav-served:dumps/$name.part webdav-served:dumps/$name
else
  rclone deletefile webdav-served:dumps/$name.part
fi
```

### Configurazione per i backup

Server, istanza dedicata:

```bash
GOMEMLIMIT=600MiB rclone serve webdav /srv/backup \
  --addr :9090 --htpasswd /etc/rclone/htpasswd \
  --vfs-cache-mode off --ignore-checksum
```

Client:

```ini
[webdav-served]
type = webdav
url = http://server:9090
vendor = rclone
user = utente
pass = *** (rclone obscure) ***
```

---

## 9. Riprodurre le misure

Server in un container limitato come l'host di destinazione, con un binario statico
(`CGO_ENABLED=0 go build -trimpath`), client rclone sull'host:

```bash
docker run -d --name wd --memory=1g --memory-swap=1g --cpus=1 \
  -v $PWD/srvtest:/t -p 18980:8080 -p 15980:5572 alpine:3.24 \
  /t/bin/rclone serve webdav /t/data --addr :8080 \
  --rc --rc-addr :5572 --rc-no-auth --log-level INFO --log-file /t/logs/wd.log \
  --server-read-timeout 10s --server-write-timeout 10s

export RCLONE_CONFIG_WD_TYPE=webdav RCLONE_CONFIG_WD_URL=http://127.0.0.1:18980 RCLONE_CONFIG_WD_VENDOR=rclone
head -c 30M /dev/urandom > f30
time rclone copyto f30 wd:f30 --bwlimit 1M --retries 1 --low-level-retries 1   # timeout di upload
time curl -s -o f30.get --limit-rate 1M http://127.0.0.1:18980/f30             # timeout di download
curl -s -o /dev/null -w '%{http_code} %{size_download}\n' -X PROPFIND http://127.0.0.1:18980/
```

- CPU: differenza di `usage_usec` in `/sys/fs/cgroup/cpu.stat` del container prima e dopo il trasferimento.
- Memoria: `memory.peak` e la riga `anon` di `memory.stat` del cgroup; heap da `rclone rc core/memstats`.
  `memory.peak` comprende la page cache dei file scritti: per i trasferimenti grandi conta la memoria anonima.
- Profili: `curl -o cpu.pprof 'http://127.0.0.1:15980/debug/pprof/profile?seconds=6'`, poi
  `go tool pprof -top`.

Ambiente delle misure: Go 1.27, client e server sullo stesso host (CPU del client non conteggiata), dati su un
bind mount; i tempi assoluti dipendono dal disco, i rapporti fra configurazioni no.

Le prove di streaming della [sezione 8](#8-backup-in-streaming-rclone-rcat) usano
`fork-doc/scripts/rcat-stream-test.sh`, che mette anche il client in un container limitato e campiona ogni secondo
memoria, `/tmp` e disco di entrambi:

```bash
T=$PWD/srvtest bash fork-doc/scripts/rcat-stream-test.sh webdav base 1024                     # stream da 1 GiB
T=$PWD/srvtest bash fork-doc/scripts/rcat-stream-test.sh webdav ic 1024 --ignore-checksum     # flag del server
T=$PWD/srvtest VENDOR=other bash fork-doc/scripts/rcat-stream-test.sh webdav spool 256        # spool nel /tmp del client
T=$PWD/srvtest PRE=256 KILL=6 LSAFTER=1 bash fork-doc/scripts/rcat-stream-test.sh webdav kill 512  # client ucciso
T=$PWD/srvtest DATA=/d SRVX="--tmpfs /d:size=96m,mode=1777" \
  bash fork-doc/scripts/rcat-stream-test.sh webdav full 256                                   # disco pieno
```
