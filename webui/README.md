# webui — interfaccia web per `rclone serve webdav`

## Scopo

Questo package aggiunge a `rclone serve webdav` l'interfaccia web di
superdufs (un fork di [dufs](https://github.com/sigoden/dufs)): un browser che apre il server vede una pagina
con navigazione delle cartelle, upload, download, ricerca, modifica dei file di testo, player multimediale
e gestione dei file invece del semplice elenco HTML. I client WebDAV e la riga di comando non sono
interessati: il protocollo WebDAV resta quello upstream.

Il package è un'aggiunta del fork: l'unica modifica a file upstream sono tre righe marcate
`// fork:webui` in `cmd/serve/webdav/webdav.go`; il resto è in `webui/`, in
`cmd/serve/webdav/webdav_webui*.go` e nei workflow generati `.github/workflows/fork-*.yml` (vedi
[CI del fork](#ci-del-fork)). Lo script `scripts/check-footprint.sh` lo verifica. Questo file è
l'unica documentazione del fork: la documentazione upstream non è modificata.

L'interfaccia è attiva per default. Resta spenta (comportamento identico a upstream) con `--webui=false`,
con `--template` oppure con `--disable-dir-list`.

## Avvio rapido

Serve un remote (o una cartella locale) su tutte le interfacce, porta 8000, aprendolo poi con un browser
su `http://host:8000/`:

```bash
rclone serve webdav --addr 0.0.0.0:8000 myremote:
```

Esempio più completo, con autenticazione, cache di scrittura (necessaria per riprendere gli upload
interrotti), link pubblici per una cartella e segreto passato con la variabile d'ambiente, così non compare
nella riga di comando:

```bash
export RCLONE_WEBUI_SECRET='una-frase-lunga-e-casuale'
rclone serve webdav --addr 0.0.0.0:8000 \
    --user alice --pass 'password' \
    --vfs-cache-mode writes \
    --webui-publink-paths /condivisi \
    myremote:
```

Anche `--pass` è visibile con `ps`: su una macchina condivisa usa `--htpasswd` con un file htpasswd.
Per usare il server solo come WebDAV, senza pagina, aggiungi `--webui=false`.

## Flag

Ogni flag ha la corrispondente variabile d'ambiente `RCLONE_<NOME>` (es. `RCLONE_WEBUI_SEARCH`) e, per
`rc`, il blocco di opzioni `webdav_webui`. L'elenco si ottiene anche con `rclone serve webdav --help`.

### Flag dell'interfaccia

| Flag | Default | Descrizione |
|------|---------|-------------|
| `--webui` | `true` | Serve l'interfaccia web ai browser che aprono una cartella |
| `--webui-search` | `true` | Abilita la ricerca ricorsiva |
| `--webui-search-max-depth` | `8` | Profondità massima della ricerca |
| `--webui-search-max-results` | `1000` | Numero massimo di risultati della ricerca |
| `--webui-search-timeout` | `30s` | Durata massima di una ricerca |
| `--webui-publink-paths` | vuoto | Percorsi (e il loro contenuto) per cui si creano link pubblici di sola lettura; `/` indica tutto |
| `--webui-secret` | vuoto | Segreto con cui sono firmati i token dell'interfaccia (sensibile) |
| `--webui-token-expiry` | `72h` (`3d` nell'help) | Durata dei token di download |
| `--webui-resume` | `false` | Ricorda la posizione di riproduzione dei file multimediali |
| `--webui-resume-dir` | vuoto | Cartella dove sono salvate le posizioni di riproduzione |
| `--webui-resume-retention` | `720h` (`1M` nell'help) | Per quanto tempo si conservano le posizioni non usate |
| `--webui-player-max-retry` | `30` | Tentativi massimi di riconnessione del player |
| `--webui-player-reconnect-interval` | `10` | Secondi tra i tentativi di riconnessione del player |
| `--webui-disk-stats` | `false` | Mostra le statistiche di spazio disco |
| `--webui-upload-settings` | `false` | Mostra le impostazioni di upload concorrenti |
| `--webui-edit-max-size` | `4Mi` | Dimensione massima di un file modificabile dall'interfaccia |
| `--webui-dir-count` | `false` | Mostra il numero di elementi delle cartelle |
| `--webui-multi-zip-max` | `200` | Numero massimo di elementi di uno zip da selezione multipla |

Note:

- `--webui-dir-count`: la colonna dimensione delle cartelle mostra il numero di elementi (al massimo 1000)
  invece di un trattino. Costa un elenco di ogni sottocartella a ogni apertura della pagina, quindi è
  spento per default; se una sottocartella non si può leggere resta il trattino.
- `--webui-search`, `--webui-disk-stats`: attivano la ricerca e le statistiche di spazio (sezione
  "Ricerca e statistiche") e i campi `allow_search` ed `enable_data_stats` dei dati della pagina.
- `--webui-upload-settings`: mostra nella pagina le impostazioni degli upload concorrenti (campo
  `show_concurrent_uploads_settings` dei dati della pagina); la scelta è del browser e il server non ha
  limiti propri.
- `--webui-player-max-retry`, `--webui-player-reconnect-interval`: sono passati al player della pagina
  `?view`, che li usa per riconnettersi quando il flusso si interrompe.
- `--webui-token-expiry`: un valore minore o uguale a zero fa scadere subito i token di download.
- `--webui-resume-retention`: `0` conserva le posizioni di riproduzione per sempre.

### Flag di rclone che influenzano l'interfaccia

| Flag | Effetto sull'interfaccia |
|------|--------------------------|
| `--read-only` | La pagina nasconde upload e cancellazione (`allow_upload` e `allow_delete` falsi); un `PUT` dal browser passa all'upstream, che lo rifiuta |
| `--disable-zip` | La pagina non offre lo scarico come archivio (`allow_archive` falso) e `?zip`, `?zip=multi` rispondono `403`, come lo zip upstream (`?download=zip`) che lo stesso flag disattiva |
| `--template` | L'interfaccia resta spenta: si ottiene il comportamento upstream con il template scelto |
| `--disable-dir-list` | L'interfaccia resta spenta |
| `--baseurl` | Prefisso di tutti gli URL, anche quelli degli asset e dei link pubblici (`<baseurl>/@public/...`); i redirect lo mantengono |
| `--user`, `--pass`, `--htpasswd` | Autenticazione del server. Con l'autenticazione attiva la pagina chiede i token di download (`?tokengen`), conosce l'utente (`CHECKAUTH`) e può fare il logout (`LOGOUT`) |
| `--user-from-header` | Conta come autenticazione: l'utente è quello del header e la pagina chiede i token di download. Di solito il login vero lo fa il reverse proxy davanti a rclone, quindi gli URL con `?token=` e `@public/` funzionano senza credenziali solo se il proxy li lascia passare senza autenticazione |
| `--client-ca` | Conta come autenticazione (utente dal certificato), ma il TLS richiede un certificato client valido a ogni connessione: token di download e link pubblici funzionano solo per client che hanno già un certificato |
| `--realm` | Realm dell'autenticazione Basic, usato anche dalla risposta a `LOGOUT` (default `rclone`) |
| `--auth-proxy` | La VFS dipende dall'utente: i token di download e i link pubblici sono spenti (la pagina scarica con le credenziali del browser) |
| `--vfs-cache-mode` | Con `off` o `minimal` un upload interrotto cancella il file parziale e la ripresa (`PATCH` append) non è disponibile; con `writes` o `full` il parziale è tenuto e l'upload riprende |
| `--no-modtime` | Le date di modifica sono `0` e la pagina lascia vuota la colonna |
| `--allow-origin` | Consente a un'altra origine il preflight CORS dei metodi di scrittura (vedi "Sicurezza") |
| `--response-header` | Gli header impostati valgono anche per le risposte dell'interfaccia, compresi asset, download con token e link pubblici serviti senza credenziali |
| `--dir-cache-time`, `--vfs-disk-space-total-size`, `--vfs-used-is-size` | Cache degli elenchi e valori delle statistiche di spazio (vedi "Ricerca e statistiche") |

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

### Scarico come zip

`GET <cartella>/?zip` scarica la cartella come archivio, chiamato `<nome cartella>.zip` (`root.zip` per la
radice); `GET <cartella>/?zip=multi&paths=["a","b/c"]` scarica la selezione fatta nella pagina, chiamata
`selection.zip`. `paths` è un array JSON di percorsi relativi alla cartella (al massimo 8192 caratteri e
`--webui-multi-zip-max` elementi); percorsi con `.` o `..` sono rifiutati con `400`, un percorso inesistente
dà `404`. Con `--disable-zip` entrambi rispondono `403`. L'archivio è scritto mentre si legge: un errore a
metà lascia un file senza elenco finale (non confondibile con uno completo) e si legge solo nel log.

## Comportamento con client WebDAV

Le operazioni di scrittura della pagina usano i metodi WebDAV normali, ma alcune hanno bisogno di regole
diverse da quelle di `x/net/webdav`. La UI le applica solo alle richieste con intestazione `Origin`, che i
browser aggiungono alle richieste `PUT`, `PATCH` e `MOVE` fatte da script e che i client WebDAV non
inviano: per questi ultimi le risposte restano quelle upstream (un test le confronta con un server a UI
spenta).

- `CHECKAUTH` e `LOGOUT` (metodi della pagina, sconosciuti all'upstream): `CHECKAUTH` risponde con il nome
  dell'utente autenticato (vuoto senza autenticazione; `?login` senza utente chiede le credenziali);
  `LOGOUT` risponde `401` con `WWW-Authenticate: Basic realm="<--realm>"` (default `rclone`) così il
  browser dimentica le credenziali.
- `PUT` dal browser crea le cartelle mancanti del percorso (upstream risponde `409`) e risponde `201`.
  Se il corpo arriva incompleto: con `--vfs-cache-mode` `off` o `minimal` il file parziale è cancellato;
  con `writes` o `full` è tenuto perché la pagina possa riprendere l'upload. Con `--read-only` la
  richiesta passa all'upstream.
- `PATCH` con `X-Update-Range: append` (ripresa di un upload) aggiunge il corpo in coda al file e
  risponde `204`. Richiede `--vfs-cache-mode writes` o `full`: altrimenti `405`.
- `MOVE` dal browser senza intestazione `Overwrite` verso una cartella che esiste già risponde
  `412 Destination folder exists`. La pagina non chiede conferma per le cartelle e l'upstream, in
  assenza di `Overwrite`, sovrascrive: senza questo controllo spostare un elemento su una cartella
  esistente la cancellerebbe con tutto il contenuto.
- `?edit` e `?view` su un file aprono l'editor o il player della pagina; un file è modificabile se non
  supera `--webui-edit-max-size` e i primi 1024 byte sono testo UTF-8 senza byte nulli.

Le regole di sicurezza che riguardano i browser (sandbox dei file attivi, preflight CORS) sono nella
sezione "Sicurezza"; i limiti di questi comportamenti sono in "Limiti noti".

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

## Ricerca e statistiche

### Ricerca

Il campo di ricerca della pagina apre `<cartella>/?q=<testo>`, che risponde come l'elenco della cartella
(stessi dati, `?json` e HTML) con i soli elementi il cui nome contiene il testo, senza distinguere maiuscole,
cercati anche nelle sottocartelle. I nomi sono percorsi relativi alla cartella corrente (`x/match2.txt`);
l'ordine segue `?sort` e `?order` come nell'elenco. Le cartelle trovate hanno sempre dimensione `-1`, anche
con `--webui-dir-count`. Un testo vuoto o di soli spazi dà l'elenco normale, come `--webui-search=false`
(in quel caso `allow_search` è falso e `?q` è ignorato). Come l'elenco, la ricerca risponde solo ai browser
e a `?json`: un client da riga di comando che invia `?q` senza `?json` passa all'upstream.

| Flag | Default | Effetto |
|------|---------|---------|
| `--webui-search` | `true` | Abilita la ricerca |
| `--webui-search-max-depth` | `8` | Livelli di sottocartelle esplorati: con `0` solo la cartella corrente |
| `--webui-search-max-results` | `1000` | Numero massimo di risultati |
| `--webui-search-timeout` | `30s` | Durata massima; vale anche la chiusura della connessione da parte del browser |

La ricerca scende per livelli. Quando raggiunge un limite (profondità, risultati o tempo) o il browser chiude
la richiesta si ferma e risponde `200` con i risultati trovati fin lì: un elenco parziale non è segnalato
come errore. Le sottocartelle che non si riescono a leggere sono saltate (se ne scrive nel log a livello
debug). I limiti sono usati come indicati: `0` risultati o `0s` di tempo danno un elenco vuoto.

Su un remote la ricerca costa un elenco di ogni cartella visitata (una richiesta al backend per cartella,
se non sono già in cache), quindi su remote grandi o lenti conviene tenere bassi profondità e timeout. Una
singola lettura già avviata non è interrotta dal timeout: la ricerca si ferma alla sua conclusione.

### Statistiche di spazio

Con `--webui-disk-stats` la pagina mostra lo spazio usato e libero, chiedendolo con `GET <cartella>/?disk-stats`
(da qualsiasi client, senza `Accept: text/html`). La risposta è `200` con
`{"disk_usage": <usato>, "disk_total": <totale>, "disk_available": <libero>}` in byte e
`Cache-Control: no-cache`; un valore sconosciuto è `null`, e se sono tutti sconosciuti la risposta è `404`
con `{"error":"Disk usage unknown"}`. Senza il flag la risposta è `404`.

I valori vengono dal metodo `About` del backend (come `rclone about` e `rclone mount`), tramite la VFS: se il
backend non lo supporta la VFS riporta uso `0` e uno spazio libero fittizio molto grande, e la pagina mostra
"Non disponibile". `--vfs-disk-space-total-size` fissa il totale; `--vfs-used-is-size` calcola l'uso sommando
le dimensioni di tutti i file, il che costa una visita completa del remote. I valori sono tenuti in cache
per `--dir-cache-time`.

## Link di download e link pubblici

### Link di download con token

Con l'autenticazione attiva la pagina rende scaricabili senza credenziali i file e gli zip delle cartelle
con un token firmato. Il browser chiede il token con `GET <percorso>?tokengen` (con le credenziali
dell'utente; la risposta è il solo token, `text/plain` con `Cache-Control: no-store`, e `404` se il percorso
non esiste) e usa poi `<percorso>?token=<token>`. Il token vale per quel file, o per lo zip di quella
cartella (`?zip` e `?zip=multi`), e non per altro: non per i figli di una cartella, non per un altro file,
non per l'elenco della cartella. Vale solo per `GET` e `HEAD`.

Il token scade dopo `--webui-token-expiry` (default `72h`; un valore minore o uguale a zero fa scadere i link
subito). Un client che invia anche un header `Authorization` è giudicato dall'autenticazione, qualunque
sia il token. I token richiedono un'autenticazione propria del server e non funzionano con `--auth-proxy`,
perché con il proxy la VFS dipende dall'utente: con `--auth-proxy`, o senza autenticazione, `?tokengen`
risponde con un token vuoto e il browser scarica con le proprie credenziali.

### Link pubblici

`--webui-publink-paths` elenca i percorsi per cui la pagina mostra un link pubblico (separati da virgola; `/`
indica tutto il server, una cartella copre anche tutto ciò che contiene). Nell'elenco e nei risultati di
ricerca ogni elemento che ricade in quei percorsi ha un `publink_token`, e la colonna "Public Link" appare
quando almeno un elemento lo ha. Il link ha la forma

```
<baseurl>/@public/<token>/<percorso>
```

e si apre senza credenziali. Il token firma il percorso: per un file vale solo per quel file, per una cartella
vale per la cartella e per tutto il suo contenuto (non per le cartelle sopra né per quelle con un nome che
inizia allo stesso modo: il token di `pub` non vale per `pubx`). Il link a una cartella apre una pagina di
sola lettura (niente upload, modifica, spostamento o cancellazione, né link da creare) con il suo contenuto
e lo zip; il link a un file lo scarica, o con `?view` apre il player. Un link a una cartella senza slash finale
è ridiretto con `302` alla forma con lo slash. Il browser invia di solito all'`@public` anche le credenziali
che ha in cache: sono ignorate, e il link è servito comunque.

Un link pubblico è permanente: non ha scadenza. Si revoca togliendo il percorso da
`--webui-publink-paths` (il server risponde `403` a un token valido per un percorso non più elencato) oppure
cambiando `--webui-secret`, che invalida tutti i token. Un link per un percorso inesistente o con un token
non valido risponde `404`, senza distinguere i due casi. Le risposte portano `Referrer-Policy: no-referrer`
perché il token è nell'URL.

I link pubblici sono spenti con `--auth-proxy` (come i token di download) e quando `--webui-publink-paths`
è vuoto. Quando sono accesi il nome `@public` nella radice del server è riservato: una cartella reale con
quel nome non è leggibile con `GET` o `HEAD`. Quando sono spenti il nome non è riservato.

Con un link pubblico si legge soltanto: le richieste che non sono `GET` o `HEAD` non sono servite e non si
scrive nulla sul remote. L'unica scrittura è quella delle posizioni di riproduzione (`?resume-get`,
`?resume-save`, `?resume-clear`, sui soli file) nell'archivio locale di `--webui-resume`, chiave per
percorso, che è lo stesso usato dalle richieste autenticate.

## Sicurezza

### Pagine e file serviti

- Le pagine HTML escono con `Cache-Control: no-cache`, `X-Content-Type-Options: nosniff` e una
  `Content-Security-Policy` (`frame-ancestors 'self'; object-src 'none'; base-uri 'self'`) che vieta frame di
  altri siti, `object` e `base` esterni.
- I file `.html`, `.htm`, `.xhtml`, `.svg`, `.svgz`, `.xml`, `.xsl` sono serviti con
  `Content-Security-Policy: sandbox`: una pagina caricata sul server non può eseguire script con le
  credenziali di chi la apre. La scelta è per estensione, vedi "Limiti noti".
- Ogni percorso toccato dall'interfaccia passa dal controllo dei percorsi dell'upstream (`checkPath`,
  che rifiuta gli elementi `.` e `..`); un percorso non valido non è mai servito dall'interfaccia e la
  richiesta va all'upstream.
- Gli asset (`index.js`, `index.css`, `favicon.ico`, `overlay.js`) sono serviti senza autenticazione sotto
  `/<baseurl>/__webui_<hash>__/`: sono identici per tutti e non contengono dati del server.

### Browser e client WebDAV

Le regole speciali per le scritture (cartelle padre create da `PUT`, `PATCH` append, rifiuto di `MOVE` su una
cartella esistente) valgono solo per le richieste con intestazione `Origin`, che i browser aggiungono ai
metodi diversi da `GET` e `HEAD`. I client WebDAV (rclone, davfs2, cadaver, Windows) non la inviano e
ricevono le risposte upstream.

`MOVE` dal browser senza intestazione `Overwrite` verso una cartella esistente risponde `412 Destination
folder exists`: la pagina non chiede conferma per le cartelle e l'upstream, in assenza di `Overwrite`,
sovrascrive, cancellando la destinazione con tutto il contenuto. Un client con `Overwrite` esplicito è
trattato come upstream.

La pagina HTML è servita solo a una richiesta `GET` o `HEAD` che accetta `text/html` e il cui `User-Agent`
non è di un programma da riga di comando (vedi "Fallback per la riga di comando"). La decisione non è una
misura di sicurezza: l'accesso ai dati è sempre soggetto all'autenticazione.

### CORS e preflight

Le richieste `PUT`, `PATCH`, `MOVE`, `CHECKAUTH` da un altro sito richiedono un preflight CORS, che
riesce solo se `--allow-origin` lo consente. Un client WebDAV web servito da un'altra origine e autorizzato
con `--allow-origin` invia `Origin` e riceve quindi le regole del browser descritte sopra. Le risposte
servite prima dell'autenticazione (asset, link pubblici, token) applicano gli stessi CORS e le stesse
intestazioni di risposta del resto del server.

### Autenticazione, token e link pubblici

Le uniche richieste servite senza credenziali sono i `GET` e `HEAD` di asset, di link pubblici
`<baseurl>/@public/<token>/<percorso>` validi e di `?token=<token>` validi (sezione "Link di download e
link pubblici"). Tutto il resto, comprese le scritture su un link pubblico, richiede l'autenticazione
del server e senza risponde `401`. Un token di download vale per un solo file o per lo zip di una
sola cartella; un token di link pubblico vale per un percorso e il suo contenuto; entrambi sono firmati con
HMAC e verificati a tempo costante. I token di download hanno scadenza (`--webui-token-expiry`), i link
pubblici no: si revocano togliendo il percorso da `--webui-publink-paths` o cambiando il segreto.

### Segreto e chiavi

Le chiavi che firmano i token di download e i link pubblici derivano da `--webui-secret`. Senza segreto sono
casuali e valgono finché il processo vive: a ogni riavvio tutti i link smettono di funzionare. Per avere
link stabili imposta il segreto. Conviene farlo con la variabile d'ambiente `RCLONE_WEBUI_SECRET` e non con
il flag, perché la riga di comando è visibile agli altri utenti della macchina (`ps`). Server con utenti
o dati diversi devono avere segreti diversi; due server con lo stesso segreto che servono gli stessi percorsi
accettano i link l'uno dell'altro. I token di download e quelli dei link pubblici usano chiavi distinte
derivate dallo stesso segreto, e non si scambiano.

Il package non scrive nel log chiavi né token. I log upstream riportano però il percorso delle richieste che
arrivano all'autenticazione o al server WebDAV, per esempio un `PUT` a un URL `@public/<token>/...` (rifiutato
con `401` e registrato come "Unauthorized request", oppure autenticato): il token è nel percorso.

## Limiti noti

- **Ripresa degli upload.** Riprendere un upload interrotto (`PATCH` con `X-Update-Range: append`) richiede
  `--vfs-cache-mode writes` o `full`; con `off` o `minimal` risponde `405` e un upload incompleto lascia il
  posto a un file cancellato, quindi si ricomincia da capo.
- **Sovrascrittura interrotta.** Un upload che sovrascrive un file esistente e si interrompe fa perdere il
  contenuto originale (il file è troncato all'apertura): è lo stesso comportamento dell'upstream e di
  superdufs.
- **Lock e data di modifica.** `PUT` e `PATCH` dal browser non controllano i lock WebDAV e ignorano
  `X-OC-Mtime`, che il browser non usa.
- **Conteggio delle cartelle.** Con `--webui-dir-count` ogni apertura della pagina costa un elenco per
  ogni sottocartella; per questo è spento per default. Senza il flag le cartelle non hanno dimensione (un
  trattino).
- **Ricerca.** Costa un elenco di ogni cartella visitata e, se raggiunge un limite, dà un elenco parziale
  senza segnalarlo come errore.
- **Link non stabili senza segreto.** Senza `--webui-secret` i token di download e i link pubblici sono
  firmati con chiavi casuali e smettono di funzionare a ogni riavvio. I token di download non funzionano con
  `--auth-proxy` né senza autenticazione; i link pubblici sono spenti con `--auth-proxy`.
- **Nome `@public` riservato.** Con i link pubblici accesi una cartella reale `@public` nella radice non è
  leggibile con `GET` o `HEAD`.
- **Date delle cartelle.** La data di modifica delle cartelle è quella che riporta il backend: molti
  remote non la gestiscono in modo significativo.
- **Sandbox per estensione.** I file che il browser eseguirebbe (`.html`, `.htm`, `.xhtml`, `.svg`, `.svgz`,
  `.xml`, `.xsl`) sono isolati con `Content-Security-Policy: sandbox`, ma la scelta è per estensione: un file
  senza estensione il cui contenuto è HTML non è isolato (come in rclone upstream).
- **Percorso nei link pubblici.** Nella pagina di un link pubblico le voci del percorso in alto che stanno
  sopra la cartella condivisa (compresa la radice) puntano a URL che il token non copre e rispondono `404`;
  nella pagina `?view` di un file portano alle pagine autenticate del server, che chiedono le credenziali.
- **Posizioni di ripresa.** Sono per remote e percorso e condivise da tutti gli utenti dello stesso remote;
  rinominare o spostare un file ne perde la posizione.
- **Zip.** Un errore dopo l'inizio dello scarico lascia un archivio incompleto (senza elenco finale) e si
  legge solo nel log.
- **Log.** I log upstream riportano il percorso delle richieste, e un token di link pubblico è nel
  percorso (vedi "Sicurezza").

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
| `webui/write.go` | Scritture dei browser: `CHECKAUTH`, `LOGOUT`, `PUT`, `PATCH` append, guardia del `MOVE` |
| `webui/zip.go` | Scarico come zip di una cartella o di una selezione |
| `webui/search.go` | Ricerca ricorsiva (`?q`) e statistiche di spazio (`?disk-stats`) |
| `webui/resume.go` | Posizioni di riproduzione (`?resume-get`, `?resume-save`, `?resume-clear`) |
| `webui/token.go` | Chiavi derivate dal segreto, token di download (`?tokengen`, `?token`) |
| `webui/public.go` | Link pubblici `@public` |
| `webui/util.go` | Funzioni comuni: intestazione `Origin`, utente, percorsi di destinazione, errori |
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
ricerca e il resto; ciò che non è interfaccia passa al server WebDAV upstream. Questo ordine dipende da
come `libhttp.NewServer` registra le opzioni e i middleware: lo proteggono `TestPreAuthBoundaries` e
`TestPublinkBoundaries`, da guardare dopo ogni sincronizzazione con upstream.

### Asset e overlay

Gli asset di superdufs sono incorporati senza modifiche: `TestAssetsMatchSource` confronta i loro sha256 con
`assets/SOURCE`. L'unico adattamento JavaScript è `overlay/overlay.js` (oggi, la dimensione delle cartelle
sconosciuta mostrata come trattino), iniettato nella pagina subito dopo il tag di `index.js`. Se il template
cambia e quel tag non compare esattamente una volta, `webui.New` fallisce: il server parte, scrive
"webui disabled" nel log e si comporta come upstream.

### Perché lo zip non usa `vfs.CreateZip`

`vfs.CreateZip` dà alle cartelle oltre il secondo livello un nome che ripete il prefisso (concatena la radice
e il percorso completo della cartella): un archivio con cartelle annidate ha nomi sbagliati. `zip.go`
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
`rerere` acceso, `webui/scripts/gen-ci.py` (rigenera la [CI del fork](#ci-del-fork) e la committa se
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

### Con superdufs

Gli asset (`assets/index.html`, `index.js`, `index.css`, `favicon.ico`) sono copiati byte per byte da
superdufs; `overlay/overlay.js` contiene le sole correzioni del fork e viene caricato dopo `index.js`.
`assets/SOURCE` registra repository, commit e sha256 degli asset. Come repository scrive
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
`overlay.js` siano sintatticamente validi e lancia `overlay/overlay_test.mjs`, che carica `index.js` e poi
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

Gli script si controllano con `bash -n webui/scripts/*.sh` e `shellcheck webui/scripts/*.sh`.

## Build

```bash
webui/scripts/build.sh              # scrive ./rclone
OUT=/usr/local/bin/rclone webui/scripts/build.sh
```

Compila con `go build -ldflags "-s -X github.com/rclone/rclone/fs.Version=<git describe>-webui"`, come il
`Makefile` upstream ma con il suffisso `-webui`: `rclone version` mostra per esempio
`rclone v1.75.0-556-g96508039e-webui`, così si riconosce il binario del fork. `go build` semplice funziona
ugualmente, solo con la versione di sviluppo.

### Con `just` e Docker

`webui/justfile` raccoglie i comandi di build. `just` cerca il justfile anche nelle cartelle superiori, quindi
si lancia da `webui/` (`just <recipe>`) o dalla radice del repository con `just -f webui/justfile <recipe>`;
le recipe lavorano sempre dalla radice. Senza argomenti `just` elenca le recipe.

```bash
just build                       # ./rclone con il Go locale (= webui/scripts/build.sh)
just image                       # immagine linux/amd64 + linux/arm64 caricata in docker
just image-for linux/arm64       # immagine per una sola piattaforma
just image-oci                   # immagine multi-arch in build/webui/rclone-webui-<versione>.oci.tar
just image-push ghcr.io/io/rclone-webui   # build e push multi-arch su un registro
just binaries                    # binari Linux, macOS, Windows (amd64, arm64) in build/webui/<os>_<arch>/
just binary darwin/arm64         # un solo binario
just clean                       # rimuove build/webui/
```

L'immagine è `rclone-webui:<versione>` (più `latest` con `just image`), con la stessa versione di `build.sh`
(`<git describe>-webui`). Il contesto di build è la cartella di lavoro, modifiche non committate comprese.

`webui/docker/Dockerfile` compila rclone con `CGO_ENABLED=0` per la piattaforma di destinazione dentro
`golang:<versione>-alpine`, dove la versione è quella della direttiva `go` di `go.mod` (segue gli
aggiornamenti upstream). L'immagine finale replica quella upstream (`alpine`, `ca-certificates`, `fuse3`,
`mailcap`, `tzdata`, utente e gruppo `rclone` 1009, `user_allow_other` in `/etc/fuse.conf`,
`ENTRYPOINT rclone`, `WORKDIR /data`, `XDG_CONFIG_HOME=/config`), ma tutti i comandi girano sulla
piattaforma dell'host: i pacchetti per l'architettura di destinazione si installano con
`apk --root --arch` senza eseguirne gli script. Per questo l'immagine arm64 si costruisce su un host amd64
senza QEMU. `Dockerfile.dockerignore` vale solo per questo Dockerfile: il `Dockerfile` upstream e la sua
build non cambiano.

Requisiti e limiti:

- serve `docker buildx`; `just image` carica un'immagine multi-arch e quindi richiede il containerd image
  store di Docker (predefinito nelle installazioni recenti). Altrimenti si usano `just image-for <piattaforma>`,
  `just image-oci` o `just image-push`;
- si compila con il tag `cmount` come le release upstream: su Windows `rclone mount` funziona con WinFsp
  (non serve cgo); su Linux l'immagine e i binari usano il `mount` in Go puro;
- i binari macOS sono compilati senza cgo, a differenza delle release upstream: `rclone mount` non c'è,
  resta `rclone nfsmount`. Non sono firmati né notarizzati, quindi Gatekeeper li mette in quarantena
  (`xattr -d com.apple.quarantine rclone`);
- i binari Windows non hanno la risorsa di versione che le release upstream aggiungono con
  `bin/resource_windows.go`;
- `just image-push` pubblica sul registro indicato: va lanciato solo quando lo si vuole davvero.

## CI del fork

I workflow upstream girano solo in rclone/rclone (ogni job controlla `github.repository`) e pubblicano sui
server di rclone. Il fork non li tocca, perché ogni modifica darebbe conflitti ai merge:
`webui/scripts/gen-ci.py` li copia in `.github/workflows/fork-*.yml` con poche sostituzioni, così il fork
esegue gli stessi job, con gli stessi test, e produce gli stessi binari e le stesse immagini. Ogni
sostituzione è ancorata a un pezzo di testo upstream: se upstream lo cambia, il generatore si ferma e
indica la regola da adattare, quindi un file generato non si allontana mai in silenzio dall'originale.
`sync-upstream.sh` rigenera i file dopo ogni merge e li committa se cambiano. Il job `webui` verifica
che siano aggiornati (`gen-ci.py --check`): non vanno modificati a mano.

| Workflow | Parte | Contenuto |
|----------|-------|-----------|
| `fork-build.yml` | push su rami e tag, pull request da altri repository, a mano | `build.yml`: build e test (`make quicktest`, `racequicktest`, test di librclone, `compile_all`) su Linux, Linux 386, macOS amd64 e arm64, Windows, altri sistemi e Go 1.26; lint (golangci-lint per 5 sistemi, govulncheck, markdownlint); build Android. In più il job `webui` e, sui tag del fork, il job `release` |
| `fork-docker-image.yml` | push su rami e tag, a mano | `build_publish_docker_image.yml`: immagine dal `Dockerfile` upstream per linux/amd64, 386, arm64, arm/v7 e arm/v6 su `ghcr.io/manprint/rclone` |
| `fork-docker-plugin.yml` | dopo le immagini di un tag, a mano | `build_publish_docker_plugin.yml`: plugin di volume Docker su Docker Hub |

Il job `webui` esegue i controlli propri del fork: workflow generati aggiornati, footprint rispetto a
`master` upstream (alla point release upstream sui rami stable del fork), `bash -n` e shellcheck sugli script, test di `serve webdav` con `RCLONE_WEBUI=false`
e `smoke.sh`. I test del package `webui` e quelli con l'interfaccia attiva fanno già parte di
`make quicktest`, su tutti i sistemi della matrice.

Dove upstream pubblica sui propri server, il fork pubblica nel repository:

- **binari**: i job con `deploy` eseguono i comandi di `make ci_beta` senza l'upload, quindi
  `bin/cross-compile.go` produce gli stessi zip, deb e rpm; invece che su beta.rclone.org finiscono negli
  artifact del workflow (`binaries-<job>`, 14 giorni). Le build Android (`make ci_upload` senza upload)
  vanno nell'artifact `android`;
- **release**: per un tag `vX.Y.Z-webui.N` il job `release` fa quello che upstream fa a mano con
  `make fetch_binaries tarball vendorball sign_upload upload_github`: raccoglie gli artifact, aggiunge i
  sorgenti e il vendor, `MD5SUMS`, `SHA1SUMS` e `SHA256SUMS`, e pubblica la GitHub Release del fork. I
  checksum sono firmati con GPG come upstream solo se esiste il secret `GPG_PRIVATE_KEY` (più
  `GPG_PASSPHRASE` se la chiave ne ha una), altrimenti sono in chiaro;
- **immagini**: gli stessi tag di upstream (`sha-…`, nome del ramo, `beta` su `main`, versione) più `X.Y`,
  `X` e `latest` per i tag `-webui.N`, che metadata-action non crea per una versione pre-release. Docker Hub
  solo con la variabile `DOCKERHUB_USERNAME` e il secret `DOCKERHUB_TOKEN`, come `<utente>/rclone`;
- **plugin Docker**: parte alla fine del workflow delle immagini di un tag, perché una release creata con
  `GITHUB_TOKEN` non avvia altri workflow; solo con Docker Hub configurato;
- `winget.yml` e `notify.yml` non hanno un equivalente: pubblicano il pacchetto winget di rclone e avvisano
  i maintainer di rclone.

### Attivazione

Una volta sola, sul repository GitHub del fork:

1. nella tab Actions abilita i workflow (GitHub li tiene spenti nei fork);
2. disattiva i workflow upstream, che nel fork salterebbero tutti i job; `winget.yml` invece non controlla
   il repository e partirebbe a ogni release. È un'impostazione del repository, i file non cambiano:

   ```bash
   for w in build.yml build_publish_docker_image.yml build_publish_docker_plugin.yml winget.yml notify.yml; do
     gh workflow disable "$w" --repo manprint/rclone
   done
   ```

3. dopo la prima pubblicazione rendi pubblico il pacchetto `ghcr.io/manprint/rclone` (nasce privato:
   Package settings → Change visibility);
4. facoltativi: variabile `DOCKERHUB_USERNAME` e secret `DOCKERHUB_TOKEN` per Docker Hub e il plugin;
   secret `GPG_PRIVATE_KEY` e `GPG_PASSPHRASE` per i checksum firmati.

### Release del fork

Si parte da una release upstream, unendo il suo tag (così `VERSION` corrisponde alla release):

```bash
git fetch upstream --tags
UPSTREAM_REF=v1.76.0 webui/scripts/sync-upstream.sh
git push origin main
git tag -a v1.76.0-webui.1 -m "rclone v1.76.0-webui.1"
git push origin v1.76.0-webui.1
```

Il tag avvia build, test, immagini e release. `rclone version` mostra `rclone v1.76.0-webui.1` e i file
si chiamano come quelli upstream con quella versione (`rclone-v1.76.0-webui.1-linux-amd64.zip`, `.deb`,
`.rpm`, …). Per una nuova release sullo stesso upstream si usa `-webui.2`.

Prima di costruire i binari di un tag, `fork-build.yml` controlla che su HEAD ci sia un solo tag, nel
formato `vX.Y.Z-webui.N`, che `VERSION` valga `vX.Y.Z` e che `vX.Y.Z` sia un tag upstream: il Makefile
prende la versione dal tag su HEAD e calcola l'ultima release (`LAST_TAG`) dal tag di `VERSION`. Il tag
del fork va quindi messo sul merge della release upstream, prima di unire il `master` successivo, che
porta `VERSION` alla versione in sviluppo. Al fork vanno spinti solo i suoi tag: un tag upstream spinto
su `origin` produrrebbe binari e immagini del fork con il numero di una release upstream.

Le point release upstream (`vX.Y.Z` con Z > 0) stanno sul ramo `vX.Y-stable`, non su `master`, quindi non
si uniscono a `main`. Per una release del fork si crea un ramo dalla release upstream e ci si applicano
le modifiche del fork, cioè la differenza tra la release `X.Y.0` e il suo tag del fork:

```bash
git fetch upstream --tags
git switch -c v1.76-webui v1.76.1
git diff --binary v1.76.0 v1.76.0-webui.1 | git apply --3way --index
python3 webui/scripts/gen-ci.py && git add .github/workflows
git commit -m "webui: add the web UI of serve webdav to v1.76.1"
git tag -a v1.76.1-webui.1 -m "rclone v1.76.1-webui.1"
git push origin v1.76-webui v1.76.1-webui.1
```

`gen-ci.py` rigenera i workflow da quelli della release upstream. Su un ramo così il job `webui` controlla
il footprint rispetto alla release upstream di `VERSION` invece che rispetto a `master`.
