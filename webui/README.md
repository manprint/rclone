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
| `--webui-publink-paths` | vuoto | Percorsi (e il loro contenuto) per cui si creano link pubblici di sola lettura; `/` indica tutto |
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
- `--disable-zip`: la pagina non offre lo scarico come archivio (`allow_archive` falso) e `?zip`,
  `?zip=multi` rispondono `403`, come lo zip upstream (`?download=zip`) che lo stesso flag disattiva.
- `--no-modtime`: le date di modifica sono `0` e la pagina lascia vuota la colonna.
- `--template`: l'interfaccia resta spenta e si ottiene il comportamento upstream con il template scelto.
- `--disable-dir-list`: l'interfaccia resta spenta.
- `--webui=false`: l'interfaccia resta spenta.
- `--webui-dir-count`: la colonna dimensione delle cartelle mostra il numero di elementi (al massimo 1000)
  invece di un trattino. Costa un elenco di ogni sottocartella a ogni apertura della pagina, quindi è
  spento per default; se una sottocartella non si può leggere resta il trattino.
- `--webui-search`, `--webui-disk-stats`: attivano la ricerca e le statistiche di spazio (sezione
  "Ricerca e statistiche") e i campi `allow_search` ed `enable_data_stats` dei dati della pagina.
- `--webui-upload-settings`: controlla soltanto il campo `show_concurrent_uploads_settings` dei dati della
  pagina; la funzione corrispondente arriva nelle fasi successive.

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
- I file `.html`, `.htm`, `.xhtml`, `.svg`, `.svgz`, `.xml`, `.xsl` sono serviti con
  `Content-Security-Policy: sandbox`: una pagina caricata sul server non può eseguire script con le
  credenziali di chi la apre.

Note e limiti:

- Un upload che sovrascrive un file esistente e si interrompe fa perdere il contenuto originale (il file
  è troncato all'apertura): è lo stesso comportamento dell'upstream e di superdufs.
- `PUT` e `PATCH` dal browser non controllano i lock WebDAV e ignorano `X-OC-Mtime`, che il browser
  non usa.
- Le richieste `PUT`, `PATCH`, `MOVE`, `CHECKAUTH` da un altro sito richiedono un preflight CORS, che
  riesce solo se `--allow-origin` lo consente. Un client WebDAV web servito da un'altra origine e
  autorizzato con `--allow-origin` invia `Origin` e riceve quindi le regole del browser descritte sopra.

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

### Limiti noti

I file che il browser eseguirebbe (`.html`, `.htm`, `.xhtml`, `.svg`, `.svgz`, `.xml`, `.xsl`) sono serviti
con `Content-Security-Policy: sandbox`, così gli script non raggiungono il server con le credenziali
dell'utente. La scelta è per estensione: un file senza estensione il cui contenuto è HTML non è isolato
(come in rclone upstream).

Nella pagina di un link pubblico le voci del percorso in alto che stanno sopra la cartella condivisa
(compresa la radice) puntano a URL che il token non copre e rispondono `404`; nella pagina `?view` di un
file portano alle pagine autenticate del server, che chiedono le credenziali.

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
