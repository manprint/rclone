# `rclone serve s3` — miglioramenti per host piccoli e oggetti grandi

Questo fork migliora `rclone serve s3` per l'uso su molte macchine con **1 GB di RAM,
1 CPU e storage nell'ordine dei TB** (tipicamente remote `local`), dove i client caricano
backup anche da **200 GB** con upload multipart.

Le modifiche al codice sono nel backport `webui/backports/07-serve-s3-small-hosts.patch`. Tutti i comandi e i numeri qui sotto sono
stati verificati con queste modifiche (dettagli in [Misure](#misure)).

---

## 1. Cosa è stato fatto

| # | Problema trovato | Effetto prima | Fix | Commit |
|---|---|---|---|---|
| 1 | `HEAD`/`GET` ricalcolavano l'MD5 leggendo **tutto** l'oggetto, anche quando l'ETag era già noto dall'upload | ~18 s in più per ogni HEAD di un oggetto da 10 GiB su `local` (rclone fa HEAD dopo ogni upload) | Si usa l'ETag tenuto in memoria; si fa l'hash solo se non è noto | `eb8610b76` |
| 2 | Upload multipart lunghi falliti con `InvalidPart`: se il client andava in timeout sulla richiesta che stava scrivendo le parti in buffer e la reinviava, la `Complete` arrivava prima che quelle parti fossero scritte | Backup da 200 GB con `--timeout 5m` di default potenzialmente **fallito alla fine** | `CompleteMultipartUpload` aspetta che le parti accettate siano scritte (`waitPumped`) | `35d272ce6` |
| 3 | La richiesta di una parte restava aperta finché il server scriveva anche tutte le parti in buffer dopo di essa (potenzialmente per tutto l'upload) | Timeout lato client e reinvii inutili; latenza massima per parte pari al timeout (10 s nel test) | Le parti in buffer sono scritte **in background**: la risposta arriva appena la propria parte è scritta. Se la scrittura in background fallisce, l'upload viene annullato e nulla viene committato | `43beb7182` |
| 4 | Le dimensioni delle parti di ogni oggetto multipart erano tenute come `[]int64` | Fino a 80 KiB per oggetto (10 000 parti), moltiplicati per `--metadata-max-objects` | Codifica run-length: un upload a parti uguali occupa pochi byte | `e567bde34` |
| 5 | I file temporanei `.rclone_temp_*` lasciati da un crash/kill restavano per sempre | Spazio perso (anche centinaia di GB per un backup interrotto) | Nuova opzione `--temp-object-expiry`: all'avvio, in background, rimuove i temporanei più vecchi della soglia | `c6fa407cd` |
| 6 | Su `local` (MD5 = lettura del file), dopo la scadenza della dir-cache VFS (5 min di default) ogni listing/HEAD/GET di un oggetto non caricato da questa istanza **rileggeva l'intero file** | Restore di un backup da 200 GB: lettura completa prima del primo byte, timeout del client, retry, nuova lettura... | L'MD5 calcolato è tenuto in memoria finché l'oggetto non cambia (LRU separata, non scaccia i metadati utente) | `ae3411ba0` |
| 7 | Documentazione mancante | — | Documentati: lettura completa per l'MD5 dopo un riavvio e uso di `hasher`; spazio doppio in sovrascrittura; pulizia dei temporanei | `2558131ae`, `7e65fe18e`, `c6fa407cd` |

Compatibilità: nessun flag esistente ha cambiato comportamento. `--temp-object-expiry` ha
default `0` (disattivo).

### Test aggiunti (`cmd/serve/s3`)

| Test | Cosa verifica |
|---|---|
| `TestStoredETagNotRehashed` | HEAD/GET dopo un multipart non rileggono l'oggetto |
| `TestMultipartCompleteWaitsForPump` | La `Complete` aspetta le parti ancora in scrittura (scenario del bug 2) |
| `TestMultipartCompleteWaitForPumpEnds` | Quell'attesa termina con `SlowDown` dopo il timeout o con la cancellazione della richiesta, e l'upload si completa dopo |
| `TestMultipartPartNotHeldByPump` | La risposta di una parte non aspetta le parti in buffer dopo di essa |
| `TestMultipartPumpFailure` | Un errore di scrittura in background annulla l'upload |
| `TestMultipartPumpFailureNotCommitted` | Una `Complete` in attesa fallisce e **non** committa un oggetto troncato |
| `TestMultipartForgetWhilePumping` | Lo shutdown durante una scrittura in background non lascia nulla |
| `TestPartRuns` | Codifica/decodifica run-length delle dimensioni delle parti |
| `TestRemoveStaleTempObjects` | Sono rimossi solo i temporanei (anche legacy) più vecchi della soglia, su `local` e `memory` |
| `TestTempObjectExpiry` | `--temp-object-expiry` pulisce all'avvio del server |
| `TestComputedETagKept` | L'MD5 calcolato sopravvive alla dir-cache; viene invalidato se l'oggetto cambia (anche con stessa size e mtime via S3) |

```bash
go test ./cmd/serve/s3/                                # tutta la suite (~2 min)
go test -race -run 'TestMultipart' ./cmd/serve/s3/     # parte concorrente
```

---

## 2. Come funziona (in breve)

- **PUT semplice**: su `local` scrive in `.rclone_temp_put_<uuid>` nella stessa directory,
  poi rinomina. Un PUT interrotto non tocca l'oggetto esistente.
- **Multipart**: la parte che serve allo stream (la prossima in ordine) è scritta subito sul
  backend, senza buffer. Le parti in anticipo sono tenute in RAM fino al loro turno, entro
  `--multipart-streaming-buffer-limit` per upload e `--multipart-streaming-buffer-total` per
  utente. Oltre quei limiti aspettano; dopo 1 minuto ricevono `SlowDown` e il client ritenta.
  Il file è `.rclone_temp_multipart_<uploadID>`, rinominato alla `Complete`.
- **ETag**: è l'MD5 dell'oggetto (anche per i multipart, senza il suffisso `-N` di AWS).
  - Durante l'upload è calcolato in streaming e tenuto in memoria (LRU per utente).
  - Su `local`, se non è noto (oggetto preesistente o dopo un riavvio), il primo accesso
    legge tutto il file. Da lì in poi resta in memoria finché l'oggetto non cambia.
- **Stato in memoria**: upload in corso, metadati utente, tag ed ETag noti sono **solo in
  RAM**. Dopo un riavvio gli upload in corso sono persi (il client deve ricominciare) e i
  loro temporanei diventano orfani (vedi `--temp-object-expiry`).

---

## 3. Configurazioni consigliate (host da 1 GB / 1 CPU)

### Profilo A — molti client o upload concorrenti

```bash
GOMEMLIMIT=600MiB rclone serve s3 /srv/s3 \
  --addr :8080 --auth-key ACCESS_KEY,SECRET_KEY \
  --multipart-streaming-buffer-limit 64M \
  --multipart-streaming-buffer-total 192M \
  --buffer-size 4M --use-mmap \
  --metadata-max-objects 20000 \
  --temp-object-expiry 48h
```

### Profilo B — un backup grande alla volta (es. 200 GB da stdin)

```bash
GOMEMLIMIT=600MiB rclone serve s3 /srv/s3 \
  --addr :8080 --auth-key ACCESS_KEY,SECRET_KEY \
  --multipart-streaming-buffer-limit 192M \
  --multipart-streaming-buffer-total 256M \
  --buffer-size 4M --use-mmap \
  --metadata-max-objects 20000 \
  --temp-object-expiry 48h
```

Con i **default** (`256M` per upload / `1G` per utente) su 1 GB di RAM, 3 upload concorrenti
da 16×64 MiB in volo sono stati **uccisi per OOM**. Non usare i default su host piccoli.

Significato dei flag:

| Flag | Perché |
|---|---|
| `GOMEMLIMIT=600MiB` | Il GC di Go libera memoria prima di arrivare al limite del cgroup (circa 60% della RAM) |
| `--multipart-streaming-buffer-limit` | RAM massima per le parti in anticipo di **un** upload |
| `--multipart-streaming-buffer-total` | RAM massima per le parti in anticipo di **tutti** gli upload di un utente |
| `--use-mmap` | I buffer stanno fuori dall'heap di Go e sono restituiti subito al sistema |
| `--buffer-size 4M` | Read-ahead per file aperto in lettura (default 16M) |
| `--metadata-max-objects 20000` | Limita metadati ed ETag tenuti in RAM (default 100 000) |
| `--temp-object-expiry 48h` | All'avvio rimuove i temporanei orfani più vecchi di 48 h (spiegato più sotto) |
| `--ignore-checksum` (opzionale) | Salta la verifica dopo la scrittura, che su `local` rilegge il file. Circa **+50% di throughput** in upload (misurato). L'ETag MD5 resta calcolato |

Regole per scegliere i valori:

- `buffer-limit ≥ (concorrenza client − 1) × chunk`: le parti in anticipo non aspettano.
  Se è più basso funziona comunque: le parti aspettano il loro turno (backpressure).
- `buffer-total ≥ upload concorrenti × buffer-limit`, altrimenti c'è backpressure tra gli
  upload.
- RAM anonima misurata ≈ `buffer-total` + circa 30–40 MiB.

### Con `hasher`: ETag persistente (consigliato su `local` con oggetti grandi)

Senza `hasher`, dopo un riavvio il primo listing/HEAD/GET di ogni oggetto legge **tutto**
il file per l'MD5. Un listing di un bucket con TB di backup legge TB. Con `hasher`
gli MD5 degli oggetti caricati via S3 sono salvati in un DB (bolt) e sopravvivono ai riavvii.

`rclone.conf`:

```ini
[s3data]
type = hasher
remote = /srv/s3
hashes = md5
max_age = off
```

```bash
GOMEMLIMIT=600MiB rclone serve s3 s3data: --cache-dir /var/lib/rclone-cache \
  --addr :8080 --auth-key ACCESS_KEY,SECRET_KEY \
  --multipart-streaming-buffer-limit 64M --multipart-streaming-buffer-total 192M \
  --buffer-size 4M --use-mmap --metadata-max-objects 20000 --temp-object-expiry 48h
```

Il DB sta in `--cache-dir` (sottocartella `kv`): deve essere su disco persistente, non su
tmpfs. Misurato: HEAD di un oggetto da 10 GiB dopo un riavvio **0.1 s** con `hasher`,
contro **16.5 s** senza.

In alternativa, se i client non usano l'ETag, `--etag-hash ""` disattiva l'MD5 e il costo
che comporta.

### Auth proxy

```bash
rclone serve s3 --auth-proxy /usr/local/bin/s3-auth-proxy \
  --multipart-streaming-buffer-limit 64M --multipart-streaming-buffer-total 192M ...
```

I limiti di buffer e `--metadata-max-objects` valgono **per utente**. Con N utenti attivi
insieme la RAM sale a circa N × `buffer-total`: dimensionare di conseguenza.
`--temp-object-expiry` è ignorato con `--auth-proxy`, perché i remote sono noti solo al
login: usare il cron descritto più sotto.

### systemd (esempio)

```ini
[Service]
Environment=GOMEMLIMIT=600MiB
ExecStart=/usr/local/bin/rclone serve s3 /srv/s3 --addr :8080 \
  --auth-key ACCESS_KEY,SECRET_KEY \
  --multipart-streaming-buffer-limit 64M --multipart-streaming-buffer-total 192M \
  --buffer-size 4M --use-mmap --metadata-max-objects 20000 --temp-object-expiry 48h
MemoryMax=900M
Restart=on-failure
```

---

## 4. Valori consigliati per dimensione dei file

Valori per un host da 1 GB / 1 CPU e client rclone. "Server" si riferisce al profilo A o B
della sezione 3.

| Dimensione file | Chunk client | Concorrenza client | Server | Note |
|---|---|---|---|---|
| < 200 MB (tanti file piccoli) | nessuno: PUT singolo (`--s3-upload-cutoff` 200M di default) | `--transfers 8 --checkers 16` | A | 2000 file da 256 KiB in 7.8 s, RAM anonima 37 MiB |
| 200 MB – 5 GB | `--s3-chunk-size 16M` | `--s3-upload-concurrency 4` | A | (4−1)×16M = 48M, sotto i 64M |
| 5 – 50 GB | A: `--s3-chunk-size 32M`; B: `64M` | A: `--s3-upload-concurrency 3`; B: `4` | A (più upload) o B (uno alla volta) | (3−1)×32M = 64M e (4−1)×64M = 192M: nessuna attesa. Valori più alti funzionano con backpressure: 3 upload con 16×64M in volo ciascuno su A, 9 GiB in 89 s, RAM anonima 226 MiB, 0 SlowDown |
| 50 – 200+ GB | `--s3-chunk-size 64M` (stream: **obbligatorio**, vedi sotto) | `--s3-upload-concurrency 4` | B, con `hasher` se `local` | stdin 5 GiB in 48 s, RAM anonima 215 MiB. 200 GiB ≈ 31 min (*estrapolato*); serve 2× spazio se sovrascrive |

**Attenzione agli upload in streaming** (dimensione sconosciuta: `tar … | rclone rcat`,
`aws s3 cp -`). Il numero di parti è al massimo 10 000, quindi la dimensione massima è
**chunk × 10 000**:

| `--s3-chunk-size` | Dimensione massima in streaming |
|---|---|
| 5M (default rclone) | **48.8 GiB**: un backup da 200 GB **fallisce** |
| 32M | 312 GiB |
| 64M | 625 GiB |
| 128M | 1.22 TiB |

Con dimensione nota (`rclone copy` di un file) rclone aumenta il chunk da solo.

---

## 5. Esempi d'uso dei client

### rclone

```ini
[myserve]
type = s3
provider = Rclone
endpoint = http://host:8080
access_key_id = ACCESS_KEY
secret_access_key = SECRET_KEY
```

```bash
# backup di file (dimensione nota)
rclone copy /data/backup.tar myserve:backups/ \
  --s3-chunk-size 64M --s3-upload-concurrency 4

# backup in streaming da 200 GB (dimensione sconosciuta): chunk obbligatorio
tar -C /data -cf - . | rclone rcat myserve:backups/data-$(date +%F).tar \
  --s3-chunk-size 64M --s3-upload-concurrency 4

# se la dimensione è nota, passarla aiuta (dimensionamento dei chunk)
rclone rcat --size 214748364800 myserve:backups/vm.img < /dev/vg/vm

# molti file piccoli
rclone sync /home myserve:home/ --transfers 8 --checkers 16

# restore
rclone copy myserve:backups/backup.tar /restore/ --multi-thread-streams 4
```

`--timeout`: il default `5m` va bene dopo questi fix. Alzarlo (es. `--timeout 30m`) solo se
si serve `local` **senza** `hasher` e si leggono oggetti grandi subito dopo un riavvio del
server: in quel caso il primo accesso legge tutto il file prima di rispondere.

### AWS CLI

```bash
aws configure set default.s3.multipart_chunksize 64MB
aws configure set default.s3.max_concurrent_requests 4

aws --endpoint-url http://host:8080 s3 cp backup.tar s3://backups/
# stream > 50 GB: --expected-size obbligatorio (byte)
tar -cf - /data | aws --endpoint-url http://host:8080 \
  s3 cp - s3://backups/data.tar --expected-size 214748364800
```

---

## 6. Pulizia dei temporanei

Un server ucciso durante un upload lascia `.rclone_temp_put_*` o
`.rclone_temp_multipart_*` (e, da versioni vecchie, `.rclone_multipart_upload_*`). Sono
nascosti dai listing S3 ma occupano spazio.

- **Automatica**: `--temp-object-expiry 48h`. All'avvio il server percorre il remote in
  background, senza bloccare le richieste, e rimuove i temporanei non modificati da più di
  48 h. Nel log compare:
  `serve s3: removed N stale temporary objects older than 2d, failed to remove 0`.
- **Manuale / cron** (anche con `--auth-proxy`):

  ```bash
  rclone delete --min-age 48h \
    --include ".rclone_temp_*" --include ".rclone_multipart_upload_*" /srv/s3
  ```

Se più server condividono lo stesso storage, la soglia deve essere maggiore del
`--multipart-expiry` di tutti (default 24h).

---

## 7. Misure

Server in `docker run --memory=1g --memory-swap=1g --cpus=1` (alpine), remote `local`.
Client rclone sullo stesso host. Dati da `/dev/zero` (upload) o `/dev/urandom` (stdin 5 GiB,
file piccoli). Le letture vengono in gran parte dalla page cache: **su dischi reali le
letture complete sono limitate dalla velocità del disco**. "RAM anonima" è il valore `anon`
di `memory.stat` del cgroup, cioè la memoria del processo senza page cache.

| Scenario | Risultato |
|---|---|
| Upload multipart 10 GiB, 64M×4, profilo A | 92.6 s ≈ 110 MiB/s, CPU 100% (limite: MD5 + verifica) |
| 200 GiB allo stesso ritmo | ≈ 31 min (*estrapolato*); con `--ignore-checksum` ≈ 20 min (*estrapolato*) |
| 3 upload concorrenti × 3 GiB, 64M×16, profilo A | 89 s, RAM anonima max 226 MiB, 0 SlowDown, 0 errori (prima e dopo il pump in background) |
| Default (256M/1G), 3 upload, 64M×16 | **OOM kill** |
| stdin 5 GiB senza `--size`, 64M×4, profilo B | 48 s, RAM anonima max 215 MiB |
| 2000 file × 256 KiB, `--transfers 8` | 7.8 s, RAM anonima max 37 MiB; listing 0.1 s |
| GET 10 GiB, stream singolo (`rclone cat`) | 24 s (≈ 430 MiB/s, da page cache), CPU 100%, RAM anonima 85 MiB |
| GET 10 GiB, `rclone copyto --multi-thread-streams 4` | 45.7 s (include scrittura e verifica MD5 lato client), RAM anonima server 39 MiB |
| HEAD 10 GiB dopo upload / dopo scadenza dir-cache | 0.0–0.1 s |
| HEAD 10 GiB dopo riavvio, `local` senza `hasher` | 16.5 s (lettura completa) |
| HEAD 10 GiB dopo riavvio + scadenza dir-cache | **16.3 s → 0.1 s** (fix 6) |
| HEAD 10 GiB dopo riavvio, con `hasher` | 0.1 s |
| Repro del bug 3 (`--bwlimit 10M`, buffer 1G, client `--timeout 10s`, 400 MiB) | latenza massima per parte **10 s (timeout + reinvio) → 1 s** |
| Kill durante un upload da 1 GiB, poi riavvio con `--temp-object-expiry 24h` | temporaneo orfano rimosso all'avvio (`removed 1 stale temporary objects older than 1d`) |
| `GOMAXPROCS` | 2 in container `--cpus=1` (Go ≥ 1.25 segue il limite del cgroup, minimo 2); 1 su una VM con 1 vCPU |

---

## 8. Limiti residui (intrinseci o by design)

- **Riavvio = upload in corso persi**: lo stato multipart è solo in RAM. Il client deve
  rifare l'upload (rclone lo fa con `--retries`). I temporanei sono puliti da
  `--temp-object-expiry`.
- **Metadati utente e tag solo in RAM**: `X-Amz-Meta-*` e tag sono persi al riavvio.
  L'mtime resta, perché è salvato come data di modifica del file. L'ETag è persistente solo
  con `hasher`.
- **Primo accesso dopo un riavvio senza `hasher`**: lettura completa del file per l'MD5,
  listing compresi. Usare `hasher` o `--etag-hash ""`.
- **Sovrascrittura**: il vecchio oggetto è sostituito solo a upload completo, quindi serve
  spazio per entrambi (200 GB sovrascritti = 400 GB).
- **CPU**: con 1 vCPU l'upload è limitato dalla CPU (≈ 110 MiB/s).
- **`Complete`**: aspetta che le parti in buffer siano scritte, al massimo
  `buffer-limit` / velocità di scrittura. Dopo 1 minuto risponde `SlowDown` e il client
  ritenta; l'upload resta valido.
- **ETag multipart**: è l'MD5 dell'oggetto intero, non il formato `md5-N` di AWS.
- **Timeout HTTP**: `--server-read-timeout` e `--server-write-timeout` (default `1h`) limitano il tempo
  senza progressi di una richiesta o di una risposta, non la sua durata (backport `08-http-progress-timeouts`,
  vedi [serve-webdav.md](serve-webdav.md#2-timeout-dei-trasferimenti)): un GET di un oggetto grande può durare
  più di un'ora. Misura con `--server-write-timeout 10s` e GET da 30 MiB a 1 MiB/s: senza il backport la
  risposta si interrompe a 20,6 s dopo 21.544.032 byte, con il backport arriva intera in 29,6 s (md5 uguale).

---

## 9. Riprodurre i test end-to-end

```bash
CGO_ENABLED=0 go build -trimpath -o /tmp/rclone .
docker run -d --name s3t --memory=1g --memory-swap=1g --cpus=1 \
  -e GOMEMLIMIT=600MiB -v /tmp/rclone:/rclone -v /srv/s3test:/data -p 18980:8080 \
  alpine:3.24 /rclone serve s3 /data --addr :8080 --auth-key accesskey1,secretkey1 \
  --multipart-streaming-buffer-limit 64M --multipart-streaming-buffer-total 192M \
  --buffer-size 4M --use-mmap --metadata-max-objects 20000

export RCLONE_CONFIG_S3T_TYPE=s3 RCLONE_CONFIG_S3T_PROVIDER=Rclone \
  RCLONE_CONFIG_S3T_ENDPOINT=http://127.0.0.1:18980 \
  RCLONE_CONFIG_S3T_ACCESS_KEY_ID=accesskey1 RCLONE_CONFIG_S3T_SECRET_ACCESS_KEY=secretkey1
mkdir -p /srv/s3test/bucket
head -c 10G /dev/zero | rclone rcat --size 10737418240 s3t:bucket/big \
  --s3-chunk-size 64M --s3-upload-concurrency 4
docker exec s3t grep -E '^(anon|file) ' /sys/fs/cgroup/memory.stat
docker rm -f s3t
```
