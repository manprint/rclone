# `rclone serve sftp` su host piccoli (1 GB / 1 CPU)

Questa guida raccoglie le verifiche su memoria e CPU di `rclone serve sftp` e i flag consigliati, per il server
e per i client. Per CPU e memoria non sono servite modifiche al codice: la CPU va quasi tutta in MD5, cifratura
SSH e chiamate di sistema per disco e rete, e i risparmi vengono dai flag. Le prove di backup in streaming hanno
invece trovato due difetti negli upload interrotti, corretti in due backport (vedi
[sezione 6](#6-backup-in-streaming-rclone-rcat)):

- `webui/backports/10-serve-sftp-dropped-uploads.patch` (`cmd/serve/sftp`);
- `webui/backports/11-vfs-failed-stream-uploads.patch` (`vfs`, vale per tutti i `serve` e per `mount`).

Tutti i numeri qui sotto sono misurati, con il metodo descritto in [Riprodurre le misure](#7-riprodurre-le-misure).

`serve sftp` non usa il server HTTP di `lib/http`, quindi il limite di un'ora sui trasferimenti corretto per
WebDAV ([serve-webdav.md](serve-webdav.md#2-timeout-dei-trasferimenti)) non lo riguarda.

---

## 1. Dove va la CPU

Profilo del server durante un trasferimento da 1 GiB con il client rclone (cifrario `aes128-gcm`):

| Upload | | Download | |
|---|---|---|---|
| `crypto/md5.block` | 40% | chiamate di sistema | 49% (lettura file 25%, scrittura socket 21%) |
| chiamate di sistema | 35% (scrittura file 25%) | `memmove` | 10% |
| lettura pacchetti SSH | 10% | cifratura AES-GCM | 9% |
| decifratura AES-GCM | 3% | | |

L'MD5 dell'upload è calcolato due volte sugli stessi byte: da `operations.Rcat` e dal backend `local`. Il client
rclone poi chiede l'hash al server con `md5sum` per verificare l'upload; il server risponde con l'hash appena
calcolato, senza rileggere il file.

---

## 2. Flag del server e dei client: CPU

CPU del server per un file casuale da 512 MiB, mediana di tre giri (md5 del file sul server sempre uguale a
quello di partenza):

| Client e flag | Upload | Download |
|---|---|---|
| rclone, default | 4375 ms | 1694 ms |
| rclone, server `--ignore-checksum` | 3188 ms | 1744 ms |
| rclone `--sftp-disable-hashcheck`, server `--ignore-checksum` | **2418 ms** | 1732 ms |
| OpenSSH `sftp`, default (`chacha20-poly1305`) | 5770 ms | 3685 ms |
| OpenSSH `sftp -c aes128-gcm@openssh.com` | 4924 ms | 2065 ms |
| OpenSSH `sftp -c aes128-gcm@openssh.com`, server `--ignore-checksum` | **2795 ms** | **1675 ms** |

- `--ignore-checksum` sul server toglie i due MD5 dell'upload. Da solo, con il client rclone, risparmia meno:
  l'hash non resta in memoria e il `md5sum` del client fa rileggere tutto il file al server. Il risparmio pieno
  (-45%) arriva con `--sftp-disable-hashcheck` (o `--ignore-checksum`) sul client.
- L'integrità in transito resta garantita da SSH: `aes128-gcm` e `chacha20-poly1305` autenticano ogni
  pacchetto. Si perde il confronto finale dell'hash fra client e file scritto.
- I client OpenSSH (`sftp`, `scp`, `rsync -e ssh`) non chiedono hash: per loro `--ignore-checksum` sul server è
  gratuito.
- OpenSSH sceglie `chacha20-poly1305`, che non ha istruzioni dedicate come AES (AES-NI): con
  `-c aes128-gcm@openssh.com` il download costa al server il 44% di CPU in meno. Il client rclone usa già
  `aes128-gcm` (primo cifrario della libreria SSH di Go).

---

## 3. Memoria

Un trasferimento singolo usa 55 - 64 MiB di memoria anonima. Ogni trasferimento ha un buffer di `--buffer-size`
(default 16M) e ogni connessione i buffer dei pacchetti SFTP.

16 upload paralleli da 64 MiB, poi 16 download (client rclone):

| Flag del server | Memoria anonima massima | Heap massimo | Upload | Download |
|---|---|---|---|---|
| default | 448 MiB | 400 MiB | 8,9 s | 10,2 s |
| `--buffer-size 4M` | 175 MiB | 154 MiB | 8,4 s | 5,1 s |
| `--buffer-size 4M --use-mmap` | 129 MiB | 84 MiB | 7,2 s | 4,4 s |

Con un solo trasferimento alla volta, invece, `--buffer-size 4M` costa CPU. Upload e poi download da 1 GiB,
due giri:

| Flag del server | CPU upload | CPU download |
|---|---|---|
| default | 6,9 s | 2,9 - 3,5 s |
| `--buffer-size 4M` | 8,0 - 8,2 s | 3,3 - 3,6 s |
| `--buffer-size 4M --use-mmap` | 7,7 - 8,0 s | 3,3 s |

Quindi: con pochi client il default; con molti trasferimenti paralleli `--buffer-size 4M --use-mmap`.

---

## 4. Configurazione consigliata (host da 1 GB / 1 CPU)

Pochi client, file grandi:

```bash
GOMEMLIMIT=600MiB rclone serve sftp /srv/data \
  --addr :2022 --authorized-keys /etc/rclone/authorized_keys \
  --ignore-checksum
```

Molti client o trasferimenti paralleli: aggiungere `--buffer-size 4M --use-mmap`.

| Flag | Perché |
|---|---|
| `GOMEMLIMIT=600MiB` | il GC parte prima del limite del cgroup |
| `--ignore-checksum` | upload senza i due MD5 del server; su `local` il confronto fra i due non verifica niente |
| `--buffer-size 4M --use-mmap` | 16 trasferimenti paralleli: 129 MiB invece di 448 MiB |
| `--vfs-cache-mode off` (default) | gli upload vanno direttamente sul disco, senza copia in cache |

Client:

```bash
# rclone: niente md5sum dopo l'upload (il server con --ignore-checksum dovrebbe rileggere il file)
rclone copy /dati sftp-served:backup --sftp-disable-hashcheck

# OpenSSH: cifrario AES-GCM invece di chacha20
sftp -c aes128-gcm@openssh.com -P 2022 utente@host
scp -c aes128-gcm@openssh.com -P 2022 file utente@host:
rsync -e 'ssh -c aes128-gcm@openssh.com -p 2022' -a dati/ utente@host:dati/
```

Con un remote vero dietro il server (S3, Drive, ...) `--ignore-checksum` toglie invece il controllo fra rclone e
il remote dopo l'upload: lì va valutato.

---

## 5. Comportamenti verificati

- Il server SFTP elabora i pacchetti con 8 worker, quindi le scritture possono arrivare fuori ordine; con
  `--vfs-cache-mode off` la VFS le rimette in ordine aspettando fino a `--vfs-write-wait` (1s). Su tutti gli
  upload da 512 MiB e 1 GiB, con rclone e con OpenSSH: nessun errore nel log e md5 identico.
- L'unica riga `ERROR` nei log delle prove è `SSH login failed: EOF`, prodotta dal controllo della porta
  dello script di prova (connessione TCP chiusa senza login).

---

## 6. Backup in streaming (`rclone rcat`)

Caso d'uso: da una VM con 3 GB liberi, `sudo tar -cpzf - mydir | rclone rcat sftp-served:dumps/mydir.tar.gz`
verso un server da 1 GB / 1 CPU, con un archivio da 27 GB. Né il client né il server devono tenere l'archivio
in RAM o in un file temporaneo.

### Verifica

Client e server in container `--memory=1g --cpus=1 --read-only`, con `/tmp` in un tmpfs da 64 MiB: qualunque
copia temporanea dello stream fallirebbe con `no space left on device`. Stream casuale da 1 GiB, md5 del file
sul server uguale a quello dello stream:

| Flag | Tempo | CPU server | Memoria anonima server | Memoria anonima client | CPU client (rclone) |
|---|---|---|---|---|---|
| default | 27 s | 10,2 s | 37 MiB | 28 MiB | 6,1 s |
| client `--sftp-disable-hashcheck` | 23 s | 9,5 s | 38 MiB | 26 MiB | 3,9 s |
| client `--sftp-disable-hashcheck`, server `--ignore-checksum` | 21 s | 5,7 s | 36 MiB | 29 MiB | 3,6 s |

- `/tmp` resta vuoto su client e server (24 KiB sul server sono le chiavi host); sul disco del server c'è un
  solo file, grande quanto la parte ricevuta.
- La memoria, misurata fino a 1 GiB, è piatta: con 512 MiB era 38 MiB sul server e 22 MiB sul client. Su
  entrambi i lati i dati passano per buffer di dimensione fissa, quindi per l'archivio da 27 GB ci si aspetta
  lo stesso; sul disco del server serve solo lo spazio del file.
- Il backend sftp carica gli stream senza copie temporanee, senza opzioni particolari sul client.
- Il tempo è limitato dalla CPU del container client, condivisa con la generazione dei dati e l'md5 di
  controllo.
- Senza `--sftp-disable-hashcheck` il client calcola l'MD5 dello stream e lo confronta con quello del server
  alla fine: è l'unico controllo dell'archivio scritto, e costa circa 2 s di CPU del client per GiB.

### Requisiti

- **Server: `--vfs-cache-mode off`** (default). Con `writes` o `full` l'upload viene scritto prima nella cache
  e poi copiato nella destinazione: serve il doppio dello spazio, e con la cache piena il client riceve
  `no space left on device (SSH_FX_FAILURE)`.

### Errori e interruzioni

| Caso | Prima | Ora |
|---|---|---|
| client ucciso o connessione caduta a metà | il pezzo ricevuto restava salvato con il nome finale, come se fosse completo | il file parziale viene cancellato (backport 10) |
| disco del server pieno | il client riceveva `io: read/write on closed pipe`, o `can't seek in file` per le scritture arrivate fuori ordine | il client riceve `no space left on device (SSH_FX_FAILURE)` (backport 11); il file parziale viene cancellato |
| sovrascrittura fallita | il vecchio file, già cancellato dal backend `local`, restava elencato fino alla scadenza della cache delle cartelle | non viene più elencato (backport 11) |
| `tar` fallisce a metà | rclone vede solo la fine dello stream e salva l'archivio troncato come completo: lo rivela soltanto il codice di uscita della pipeline con `set -o pipefail` | invariato |

- Uno stream non viene ritentato: rclone non può rileggere lo stdin, quindi dopo un errore il backup va rifatto.
- L'upload scrive direttamente nel file finale: sovrascrivere un backup con lo stesso nome non richiede spazio
  in più, ma **se lo stream fallisce il vecchio backup è già perso**. Con 3 GB liberi e un archivio da 27 GB,
  usare nomi con la data e cancellare i vecchi backup solo dopo un backup riuscito.

Per non lasciare mai un archivio troncato con il nome definitivo, caricarlo con un nome temporaneo e rinominarlo
solo se tutta la pipeline è andata bene. Su SFTP la rinomina avviene sul server, istantanea:

```bash
set -o pipefail
name=mydir-$(date +%F).tar.gz
if sudo tar -cpzf - mydir | rclone rcat sftp-served:dumps/$name.part; then
  rclone moveto sftp-served:dumps/$name.part sftp-served:dumps/$name
else
  rclone deletefile sftp-served:dumps/$name.part
fi
```

Server per i backup: la configurazione della [sezione 4](#4-configurazione-consigliata-host-da-1-gb--1-cpu),
con la cache spenta.

### Test aggiunti

- `cmd/serve/sftp/handler_test.go`, `TestFilewriteConnectionDropped`: connessione chiusa dopo 1 MiB scritto; il
  file non deve esistere.
- `vfs/write_test.go`:
  - `TestWriteFileHandleUploadFailed`: le scritture dopo il fallimento dell'upload, anche fuori ordine,
    restituiscono l'errore dell'upload;
  - `TestWriteFileHandleFailedReplace`: un file cancellato da un upload fallito che lo sostituiva non è più
    elencato, sia con l'upload fallito sia con quello abbandonato.

---

## 7. Riprodurre le misure

Server in un container limitato come l'host di destinazione, con un binario statico
(`CGO_ENABLED=0 go build -trimpath`), client sull'host:

```bash
ssh-keygen -t ed25519 -N '' -f id && cp id.pub srvtest/cache/ak
docker run -d --name sf --memory=1g --memory-swap=1g --cpus=1 \
  -v $PWD/srvtest:/t -p 18922:2022 -p 15980:5572 alpine:3.24 \
  /t/bin/rclone serve sftp /t/data --addr :2022 --user u --pass p \
  --authorized-keys /t/cache/ak \
  --rc --rc-addr :5572 --rc-no-auth --log-level INFO --log-file /t/logs/sf.log

export RCLONE_CONFIG_SF_TYPE=sftp RCLONE_CONFIG_SF_HOST=127.0.0.1 RCLONE_CONFIG_SF_PORT=18922 \
  RCLONE_CONFIG_SF_USER=u RCLONE_CONFIG_SF_PASS=$(rclone obscure p)
head -c 512M /dev/urandom > f512
rclone copyto --ignore-times f512 sf:f512 --sftp-known-hosts-file none
rclone cat sf:f512 --sftp-known-hosts-file none > /dev/null
echo "put f512 f512" | sftp -c aes128-gcm@openssh.com -i id -P 18922 \
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -b - u@127.0.0.1
```

- CPU: differenza di `usage_usec` in `/sys/fs/cgroup/cpu.stat` del container prima e dopo il trasferimento.
- Memoria: riga `anon` di `memory.stat` del cgroup; heap da `rclone rc core/memstats`.
- Profili: `curl -o cpu.pprof 'http://127.0.0.1:15980/debug/pprof/profile?seconds=6'`, poi
  `go tool pprof -top`.

Ambiente delle misure: Go 1.27, client e server sullo stesso host (CPU del client non conteggiata), dati su un
bind mount; i tempi assoluti dipendono dal disco e dalla CPU, i rapporti fra configurazioni no.

Le prove di streaming della [sezione 6](#6-backup-in-streaming-rclone-rcat) usano
`fork-doc/scripts/rcat-stream-test.sh` (uso e variabili nell'intestazione dello script, esempi in
[serve-webdav.md](serve-webdav.md#9-riprodurre-le-misure)):

```bash
T=$PWD/srvtest CLIX=--sftp-disable-hashcheck bash fork-doc/scripts/rcat-stream-test.sh sftp ic 1024 --ignore-checksum
T=$PWD/srvtest PRE=256 KILL=6 LSAFTER=1 bash fork-doc/scripts/rcat-stream-test.sh sftp kill 512
```
