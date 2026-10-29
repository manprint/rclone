# Uso della versione webui

## Scopo

Questo package aggiunge a `rclone serve webdav` l'interfaccia web di
superdufs (un fork di [dufs](https://github.com/sigoden/dufs)): un browser che apre il server vede una pagina
con navigazione delle cartelle, upload, download, ricerca, modifica dei file di testo, player multimediale
e gestione dei file invece del semplice elenco HTML. I client WebDAV e la riga di comando non sono
interessati: il protocollo WebDAV resta quello upstream. L'unica eccezione è `--webui-allow-delete=false`,
che vieta a tutti i client di cancellare e sovrascrivere (sezione "Upload senza cancellazione").

L'interfaccia è attiva per default. Resta spenta (comportamento identico a upstream) con `--webui=false`,
con `--template` oppure con `--disable-dir-list`.

## Installazione

Installa l'ultima [release](https://github.com/manprint/rclone/releases) del fork (pre-release comprese) su
Linux, macOS o BSD:

```bash
sudo -v ; curl -fsSL https://raw.githubusercontent.com/manprint/rclone/main/webui/install.sh | sudo bash
```

Con un tag si installa una release precisa, per esempio `| sudo bash -s v1.75.2-webui.4`.

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
| `--webui-allow-delete` | `true` | Permette a tutti i client di cancellare, spostare e sovrascrivere; `false` lascia solo la creazione di file e cartelle nuovi |
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
- `--webui-allow-delete`: vedi "Upload senza cancellazione"; vale solo con l'interfaccia attiva.

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
metà interrompe la connessione, così il browser segna lo scarico come fallito, e il file ricevuto resta
senza elenco finale (non confondibile con uno completo); il motivo si legge nel log.

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

## Upload senza cancellazione

`--webui-allow-delete=false` permette di caricare file e creare cartelle ma non di cancellare o sostituire
quello che c'è già, come `--allow-upload` senza `--allow-delete` in superdufs. A differenza delle altre
regole dell'interfaccia vale per **tutti i client**, browser e client WebDAV, dopo l'autenticazione:

| Richiesta | Con `--webui-allow-delete=false` |
|-----------|----------------------------------|
| `DELETE` | `403 Forbidden` |
| `MOVE` (anche una rinomina) | `403`: lo spostamento cancella l'origine |
| `PUT` su un file esistente non vuoto | `403`, il file resta com'era |
| `PUT` su un file nuovo o vuoto | Permesso: un file vuoto si può riempire una volta |
| `COPY` su una destinazione esistente | `403`; con `Overwrite: F` resta il `412` dell'upstream |
| `COPY` verso una destinazione nuova, `MKCOL`, `PATCH` append | Permessi |

L'esistenza di un file è controllata nella VFS e, se la VFS non lo conosce, anche sul backend, così un file
aggiunto da fuori dopo l'ultimo elenco in cache non è sovrascritto. Se il controllo non riesce perché il
remote dà errore o non risponde, la richiesta è rifiutata (`503`, `504` o `500`, vedi "Remote lenti o
irraggiungibili") invece di essere lasciata passare.

La pagina riceve `allow_delete` falso e nasconde cancellazione, spostamento e modifica. `?edit` su un file
non vuoto apre la pagina in sola lettura (`kind` `View`, senza salvataggio); su un file vuoto, come quello
creato con "New File", resta l'editor e il primo salvataggio riesce.

Il controllo sta nell'interfaccia: con `--webui=false`, `--template` o `--disable-dir-list` il flag è
ignorato e il server lo scrive nel log (`--webui-allow-delete=false is ignored as the web interface is off`).
Con `--read-only` tutte le scritture sono rifiutate comunque.

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
la richiesta si ferma e risponde `200` con i risultati trovati fin lì. Le sottocartelle che non si riescono a
leggere sono saltate. Se qualche cartella entro la profondità massima non è stata letta, perché dà errore o
perché il tempo è finito, i dati hanno `search_incomplete: true` e il log ne scrive il numero a livello
info; la pagina di superdufs non mostra questo campo, che si vede con `?json`. I limiti sono usati come
indicati: `0` risultati o `0s` di tempo danno un elenco vuoto.

Su un remote la ricerca costa un elenco di ogni cartella visitata (una richiesta al backend per cartella,
se non sono già in cache), quindi su remote grandi o lenti conviene tenere bassi profondità e timeout. Il
timeout vale anche per una lettura che non risponde: la ricerca smette di aspettarla e risponde, mentre la
lettura prosegue in background fino ai timeout di rclone. Finché quella lettura non finisce, una ricerca
ripetuta la ritrova in corso e la aspetta fino al proprio timeout; la cartella che la contiene si apre
normalmente (vedi "Remote lenti o irraggiungibili").

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
ricevono le risposte upstream, salvo i rifiuti di `--webui-allow-delete=false`, che valgono per tutti
(sezione "Upload senza cancellazione").

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

## Remote lenti o irraggiungibili

`serve` non ha retry propri: ogni operazione usa i retry a basso livello del backend
(`--low-level-retries`, default 10) con `--contimeout` (60s) e `--timeout` (5m di inattività) per tentativo.
`--retries` vale solo per sync e copy. Un remote che rifiuta la connessione fallisce in fretta; uno che non
risponde può tenere ferma un'operazione per minuti.

### Profilo consigliato

I default di rclone sono pensati per trasferimenti lunghi e non presidiati, dove aspettare è meglio che
fallire. Per un server che risponde a persone conviene fallire prima. `docker-serve-webdav/.env.example`
usa questo profilo:

```sh
rclone serve webdav remote: --webui \
  --contimeout 15s --timeout 1m --low-level-retries 5 \
  --dir-cache-time 1h --vfs-cache-mode full
```

| Flag | Default rclone | Profilo | Perché |
|------|----------------|---------|--------|
| `--contimeout` | `60s` | `15s` | Un host spento o dietro un firewall che scarta i pacchetti si scopre in 15 secondi invece di 60. Vale anche per l'handshake TLS |
| `--timeout` | `5m` | `1m` | Silenzio massimo del remote, prima dell'inizio della risposta e tra un blocco di dati e il successivo. Non è un limite alla durata di un trasferimento |
| `--low-level-retries` | `10` | `5` | Ogni tentativo può durare fino a uno dei due timeout: metà tentativi, metà attesa |
| `--dir-cache-time` | `5m` | `1h` | Le cartelle viste nell'ultima ora si aprono senza toccare il remote, anche se è giù |
| `--vfs-cache-mode` | `off` | `full` (o `writes`) | Gli upload finiscono su disco e arrivano al remote quando torna |

Su un remote che non risponde un'operazione resta ferma, circa, fino a 5 × 15s se il remote non accetta la
connessione e fino a 5 × 1m se la accetta ma non risponde; con i default fino a 10 × 60s e 10 × 5m. Per
tutto quel tempo aspetta anche chi apre la stessa cartella (vedi i limiti della VFS più sotto).

Quando tornare verso i default:

- `--timeout 5m` se il server copia o sposta file grandi lato server (`MOVE` e `COPY` WebDAV su S3,
  Nextcloud, ownCloud, ...), cancella cartelle grandi o carica file grandi su remote che confermano
  l'upload solo dopo averlo salvato o ricomposto (Nextcloud e ownCloud con upload a pezzi, un `PUT` WebDAV
  unico, alcuni S3 compatibili alla chiusura del multipart): il remote risponde solo a operazione finita,
  il tentativo scade (`timeout awaiting response headers`) e rclone ripete l'operazione da capo. Un upload
  dalla cache ricarica così l'intero file a ogni tentativo, senza fine.
- `--low-level-retries 10` su Google Drive, OneDrive, Dropbox, Box e altri remote con rate limit: i
  tentativi, con pause crescenti, assorbono i rifiuti per troppe richieste.
- `--contimeout 30s` o più su linee lente, satellitari o mobili.

Con il remote giù:

- `--dir-cache-time` più alto allunga la finestra in cui le cartelle già viste restano navigabili.
  Scaduto, un elenco che fallisce è un errore: la VFS non serve l'ultimo elenco noto. Le modifiche fatte
  da fuori arrivano con `--poll-interval` sui remote che lo supportano, altrimenti alla scadenza.
- Con `--vfs-cache-mode writes` o `full` un upload risponde `201` e rclone lo carica in background
  riprovando senza limite, fino a 5 minuti tra un tentativo e l'altro, anche dopo un riavvio. I file in
  attesa non escono dalla cache neanche oltre `--vfs-cache-max-size`: un guasto lungo con molti upload
  riempie il disco della cache. Nel log:
  - `vfs cache: failed to upload try #N, will retry in D: <errore>`: tentativo fallito, il prossimo tra
    `D`; `N` che cresce indica un remote giù o che rifiuta il file;
  - `vfs cache: upload succeeded try #N`: upload arrivato;
  - `vfs cache: queuing for upload in D`: upload in coda, anche quelli ripresi dopo un riavvio.

### Risposte dell'interfaccia

| Situazione | Risposta |
|------------|----------|
| Il remote dà un errore che il backend riproverebbe (rete, timeout, rate limit) | `503 Service Unavailable` |
| Il remote riporta un timeout (scadenza del contesto) | `504 Gateway Timeout` |
| Altro errore del remote | `500` |
| Il percorso non esiste | `404`, come prima |
| Upload interrotto dal client | `400 Upload incomplete`; se è il remote a rifiutarlo, il codice dell'errore del remote |

- Una pagina, un link pubblico, un download con token, `?edit`, `?view`, `?tokengen` e `?resume-save` che
  trovano il remote in errore rispondono con l'errore e non passano la richiesta all'handler upstream, che
  ripeterebbe la stessa lettura. Un link pubblico risponde `503` e non `404` finché il remote è giù.
- Le letture della VFS uguali (lo stat dello stesso percorso, l'elenco della stessa cartella) fatte nello
  stesso momento da più richieste sono condivise: con un remote che non risponde, i tentativi ripetuti del
  player e le pagine ricaricate non avviano una lettura ciascuno. Ogni richiesta smette di aspettare quando
  il browser chiude la connessione (o, per ricerca e conteggio delle cartelle, quando scade il loro tempo).
  Richieste diverse che hanno bisogno della stessa cartella, per esempio file diversi, aspettano la stessa
  lettura della VFS e ne ricevono il risultato o l'errore.
- Il player della pagina riprova da solo: `--webui-player-max-retry` (30) e
  `--webui-player-reconnect-interval` (10 secondi) ne regolano il carico durante un guasto.
- Con `--vfs-cache-mode writes` un upload riuscito nella cache risponde `201` anche se il remote è giù:
  rclone lo carica in background riprovando senza limite (fino a 5 minuti tra un tentativo e l'altro) e
  anche dopo un riavvio. La pagina non mostra gli upload ancora in attesa.

### Limiti della VFS

Sono di rclone e valgono anche per i client WebDAV:

- La lettura dell'elenco di una cartella non si interrompe se il client se ne va: prosegue fino ai timeout
  di rclone, e chi ha bisogno della stessa cartella nel frattempo aspetta quella lettura e ne riceve il
  risultato. Le pagine dell'interfaccia non hanno un limite di tempo proprio: restano in attesa finché la
  lettura finisce o il browser rinuncia. La cartella che la contiene e le altre restano utilizzabili grazie
  al backport `01-vfs-list-without-lock` ([sviluppo.md](sviluppo.md#backport-da-upstream)): nella VFS di
  rclone upstream la cartella resta bloccata durante la lettura e il blocco risale alla cartella che la
  contiene, fino alla radice di un `combine`.
- Scaduto `--dir-cache-time` un elenco che fallisce è un errore: la VFS non serve l'ultimo elenco noto.
- Con `combine` le statistiche di spazio (`--webui-disk-stats`) falliscono se un solo upstream fallisce, e
  l'errore resta in cache per `--dir-cache-time`. Con `union` un upstream che fallisce è escluso in
  silenzio dall'elenco, che la VFS tiene in cache: i suoi file sembrano spariti fino al prossimo elenco.
- `combine` non parte se un upstream non si riesce a creare all'avvio (dipende dal backend: alcuni si
  collegano già alla creazione).

### Correzioni upstream incluse

Il fork include, come [backport](sviluppo.md#backport-da-upstream), correzioni di rclone non ancora unite
upstream che riguardano `serve` e `mount`:

- la lettura di una cartella non blocca la cartella che la contiene (vedi sopra);
- un file in cache modificato e letto di continuo viene comunque caricato sul remote: prima ogni lettura
  rimandava l'upload (rclone/rclone#9979);
- con `--vfs-read-chunk-streams` 2 o più, un flusso che fallisce o una lettura annullata non bloccano più
  la lettura per sempre (rclone/rclone#10172);
- un file che il remote tronca a metà lettura dà un errore invece di essere letto, e messo in cache, come
  completo (rclone/rclone#10127).

## Limiti noti

- **Ripresa degli upload.** Riprendere un upload interrotto (`PATCH` con `X-Update-Range: append`) richiede
  `--vfs-cache-mode writes` o `full`; con `off` o `minimal` risponde `405` e un upload incompleto lascia il
  posto a un file cancellato, quindi si ricomincia da capo.
- **Sovrascrittura interrotta.** Un upload che sovrascrive un file esistente e si interrompe fa perdere il
  contenuto originale (il file è troncato all'apertura): è lo stesso comportamento dell'upstream e di
  superdufs.
- **Upload senza cancellazione.** Con `--webui-allow-delete=false` i programmi che salvano sovrascrivendo
  un file, o scrivendo un file temporaneo e poi rinominandolo (editor e suite d'ufficio su un disco WebDAV
  montato, Esplora risorse che rinomina, `rclone sync` e `rclone copy` che aggiornano o cancellano),
  ricevono `403`: si possono solo aggiungere file nuovi. Ricaricare dalla pagina un file con un nome già
  presente fallisce allo stesso modo, e un upload interrotto tenuto dalla cache (`writes` o `full`) si può
  solo completare con la ripresa della pagina (`PATCH` append), non cancellare né ricaricare da capo. Per
  la destinazione di `COPY` il controllo sul backend riguarda i
  file: una cartella creata da fuori e non ancora nella cache degli elenchi non è riconosciuta.
- **Lock e data di modifica.** `PUT` e `PATCH` dal browser non controllano i lock WebDAV e ignorano
  `X-OC-Mtime`, che il browser non usa.
- **Conteggio delle cartelle.** Con `--webui-dir-count` ogni apertura della pagina costa un elenco per
  ogni sottocartella; per questo è spento per default. Gli elenchi sono fatti in parallelo (al massimo
  `--checkers` alla volta) e la pagina li aspetta per 5 secondi: le cartelle non lette in tempo, o che danno
  errore, restano senza numero. Il limite vale per quella richiesta: la lettura di una cartella che non
  risponde prosegue in background, e la pagina successiva la ritrova in corso e la aspetta per gli stessi 5
  secondi (vedi "Remote lenti o irraggiungibili"). Senza il flag le cartelle non
  hanno dimensione (un trattino).
- **Ricerca.** Costa un elenco di ogni cartella visitata e, se raggiunge un limite, dà un elenco parziale,
  segnalato solo da `search_incomplete` quando mancano cartelle.
- **Remote lenti o irraggiungibili.** Vedi la sezione omonima: i limiti principali stanno nella VFS di
  rclone e valgono anche per i client WebDAV.
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
  Su Windows due processi rclone che usano la stessa `--webui-resume-dir` possono veder fallire, di rado,
  un salvataggio o una lettura (file in uso dall'altro processo): conviene una cartella per processo.
- **Zip.** Un errore dopo l'inizio dello scarico lascia un archivio incompleto (senza elenco finale) e si
  legge solo nel log.
- **Log.** I log upstream riportano il percorso delle richieste, e un token di link pubblico è nel
  percorso (vedi "Sicurezza").
