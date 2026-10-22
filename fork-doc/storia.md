# Storia del fork

Cronologia del lavoro che ha portato al fork, dal 21 ottobre 2026. Il dettaglio di ogni passo (decisioni,
deviazioni, comandi eseguiti e risultati) è nello [stato del piano](piano/001_plan-WebdavSuperUI/STATE.md):
qui c'è il percorso e dove trovare il resto.

Ogni passo è un'unità di lavoro con un identificativo (`0.1`, `P0`, `V001`, `T-A001`, …): il commit che la
chiude ha l'identificativo nell'oggetto (`sub-phase(0.1): …`, `task(T-A001): …`) e i trailer `PEV-Plan`,
`PEV-Unit`, `PEV-Attempt` e `PEV-Result`, e la tabella "Work ledger" dello stato ne riporta modifiche ed
evidenze.

## 1. Richiesta

La [richiesta](analisi/specifiche.md): far comparire, lanciando `rclone serve webdav --addr 0.0.0.0:8000
myremote:`, l'interfaccia di superdufs con tutte le sue funzioni (upload e download di file e cartelle,
modifica, cancellazione, spostamento, selezione multipla, link diretti, player resiliente, layout mobile e
desktop). Due vincoli: mantenere tutte le opzioni e i flag di rclone, aggiungendo e non modificando, e
poter sempre unire gli aggiornamenti upstream senza conflitti.

## 2. Analisi

L'[analisi](analisi/ANALISI_WEBUI.md) ha fissato la strategia, rimasta quella del fork:

- gli asset di superdufs (`index.html`, `index.js`, `index.css`, `favicon.ico`) incorporati byte per byte
  nel binario, così si aggiornano da superdufs con uno script;
- il contratto HTTP che la pagina usa (metodi WebDAV più una dozzina di parametri come `?zip`, `?edit`,
  `?tokengen`) implementato in Go sopra la VFS di rclone: il JavaScript non sa di parlare con rclone;
- tutto il codice in file nuovi, con tre sole righe aggiunte a un file upstream e nessuna dipendenza nuova;
- flag nuovi solo con prefisso `--webui-*`; con l'interfaccia spenta il server è identico a upstream.

## 3. Piano ed esecuzione

Il [piano 001](piano/001_plan-WebdavSuperUI/overview.md) divide il lavoro in cinque fasi, con invarianti
(nessun cambiamento con l'interfaccia spenta, suite upstream verde, footprint, percorsi controllati,
autenticazione, client WebDAV invariati, asset identici) e gate di test per ogni passo. Ogni sotto-fase è
stata implementata, provata con test propri e sul binario reale, e rivista prima di chiudere la fase.

| Fase | Contenuto | Sotto-fasi | Chiusura |
|------|-----------|------------|----------|
| 0 | Fondamenta e listing | 0.1 struttura, flag e asset di superdufs; 0.2 pagina con l'elenco della VFS | `8ea53877e` |
| 1 | Operazioni dal browser, editor, ripresa | 1.1 upload, ripresa degli upload, login, spostamenti sicuri; 1.2 pagine `?edit` e `?view`, sandbox dei file attivi; 1.3 posizioni di riproduzione | `6e4a719eb` |
| 2 | Zip, ricerca, statistiche | 2.1 zip di cartelle e selezioni; 2.2 ricerca per nome e spazio disco | `4d1c84ccc` |
| 3 | Link | 3.1 link di download a scadenza; 3.2 link pubblici permanenti | `74d85c895` |
| 4 | Contratto, prove, documentazione | 4.1 blocco dell'API usata dalla pagina, smoke test su binario reale; 4.2 documentazione, script di sincronizzazione e build | `97fbecc48` |

Una sola deviazione tecnica (revisione 1.1): lo zip non usa `vfs.CreateZip`, che sbaglia i nomi delle
cartelle annidate; correggerlo avrebbe toccato un file upstream, quindi il fork costruisce l'archivio da sé
(vedi [sviluppo.md](sviluppo.md#perché-lo-zip-non-usa-vfscreatezip)).

## 4. Audit e correzioni

L'[audit V001](piano/001_plan-WebdavSuperUI/verify/verify_001_2026-10-21.md) ha ripetuto tutti i gate e
rivisto il codice: esito PASS WITH FINDINGS, tre rilievi minori, tutti risolti.

- V001-F1: i download serviti senza credenziali (token e link pubblici) non trattenevano la VFS. Corretto da
  V001-C1 (`0e09d5765`) con il test `TestPreAuthHoldsVFS`; la fase 3 è stata rivalidata (`efcb3e528`).
- V001-F2: un commit successivo alla chiusura non era registrato in un'unità; registrato.
- V001-F3: intestazione dello stato non aggiornata; aggiornata.

## 5. Build con `just` e Docker (T-A001)

`webui/justfile` e `webui/docker/Dockerfile` (`67dfded8d`): immagine Docker linux/amd64 e linux/arm64
uguale a quella upstream ma costruita senza QEMU, e binari per Linux, macOS e Windows compilati in Docker.
Vedi [build.md](build.md).

## 6. Pubblicazione su GitHub (T-A002)

Il fork è pubblicato come fork GitHub di rclone/rclone su https://github.com/manprint/rclone, ramo `main`,
che segue `master` di upstream (remote `origin` = il fork, `upstream` = rclone/rclone). Prima della
pubblicazione sono stati tolti dai file e dalla storia dei commit del fork i riferimenti a server e percorsi
privati (`1ef5d8ec5`): la storia dei soli commit del fork è stata riscritta con `git filter-repo`, i commit
upstream e la base sono rimasti identici.

## 7. CI del fork (T-A003, B-A001)

I workflow upstream girano solo in rclone/rclone e pubblicano sui server di rclone. `webui/scripts/gen-ci.py`
li copia nei workflow `fork-*.yml` (`d18d74e02`), con gli stessi test, gli stessi binari e le stesse
immagini, pubblicati negli artifact, nelle GitHub Release dei tag `vX.Y.Z-webui.N` e su
`ghcr.io/manprint/rclone`. Prima della pubblicazione i passi nuovi sono stati provati in locale: build dei
binari e release su un tag di prova, controlli del tag, footprint su `main` e su un ramo di point release,
una release di prova su `v1.75.2`.

Le stesse prove hanno trovato un difetto (B-A001, `4f9018b56`): i runner Windows estraggono i file con
`core.autocrlf=true`, che cambiava le fini riga degli asset; i test sarebbero falliti e i binari Windows
avrebbero incorporato asset diversi. Lo risolve `webui/.gitattributes`. La procedura per le point release
upstream è stata resa indipendente dai tag precedenti del fork (`b4f5c514b`).

Il 21 ottobre 2026 Actions è stato abilitato sul fork e i cinque workflow upstream disattivati. Esito del
primo run (commit `b4f5c514b`): immagini pubblicate su `ghcr.io/manprint/rclone` per le cinque piattaforme
(tag `main`, `beta`, `sha-b4f5c51`), con il pacchetto già pubblico; verdi i job `webui`, `linux`,
`linux_386`, `go1.26`, `mac_arm64`, `other_os`, `lint` e `android-all`. `mac_amd64` non è partito perché
GitHub non ha assegnato un runner macOS (capacità). `windows` ha trovato un difetto vero (B-A002): su
Windows un file aperto non si può sostituire né cancellare, e salvataggi e letture contemporanee della
stessa posizione di riproduzione fallivano. Lo risolve un lock sui file del `resumeStore`.
Nel run successivo (commit `8f56e3629`) `windows` è verde, come `mac_amd64`; `mac_arm64`, verde nel primo
run, non ha avuto un runner in tre tentativi (limite di capacità di GitHub per macOS arm64).

## 8. Documentazione in `fork-doc/` (T-A004)

Tutta la documentazione, l'analisi e il piano sono stati riuniti in questa cartella, che upstream non ha:
nessun conflitto possibile ai merge. Il piano stava prima in `docs/plans/`, dentro la cartella `docs/` di
upstream.

## 9. Prima release (T-A005, B-A003)

La prima release del fork parte dalla point release upstream v1.75.2, sul ramo `v1.75-webui` creato con la
procedura di [ci-e-release.md](ci-e-release.md#release-del-fork): le modifiche del fork applicate sul tag
upstream e la CI rigenerata dai workflow di v1.75.2.

Il primo tag, `v1.75.2-webui.1`, ha confermato i controlli del tag e la build dei binari su GitHub e ha
pubblicato le immagini, ma non la release: `lint`, che usa l'ultima versione di golangci-lint, ha segnalato
due `errcheck` in `cmd/serve/rc.go`, codice upstream di v1.75.2 corretto solo su `master`, e il job
`release` dipendeva da `lint` (B-A003). Upstream pubblica i binari dai job di build senza aspettare il lint,
quindi ora `release` aspetta solo build, Android e `webui` (`4a0c15b87` su `main`, `2fceae105` sul ramo).
Invece di spostare il tag già pubblicato si è fatta una nuova release, `v1.75.2-webui.2`; dopo la release
il tag `v1.75.2-webui.1` e la sua immagine sono stati cancellati.

Il run di `v1.75.2-webui.2` ha pubblicato al primo tentativo
[la release](https://github.com/manprint/rclone/releases/tag/v1.75.2-webui.2): 51 file, gli stessi della
release upstream v1.75.2 con la versione del fork (zip, deb e rpm per piattaforma, sorgenti, vendor,
checksum, `version.txt`), e le immagini `1.75.2-webui.2`, `1.75`, `1` e `latest` su
`ghcr.io/manprint/rclone`. I checksum corrispondono e il binario Linux scaricato stampa
`rclone v1.75.2-webui.2` e serve l'interfaccia. `lint` resta rosso sui tag di v1.75, per il codice upstream.

## 10. Docker compose per `serve webdav` (T-A006)

Su richiesta dell'utente la cartella [`docker-serve-webdav/`](../docker-serve-webdav/README.md), nella
radice del repository, avvia l'immagine `ghcr.io/manprint/rclone` in modalità `serve webdav` con Docker
Compose: `docker-compose.yml` senza valori propri, `.env.example` con tutte le variabili (flag upstream e
`--webui-*`, default ottimizzati per elenchi veloci e streaming video), un template di `rclone.conf` e una
guida che confronta anche le opzioni di superdufs con quelle del fork. Prima di crearla si è verificato che
upstream non abbia quel percorso (`master`, tag e storia), che è stato aggiunto al footprint di
`check-footprint.sh` ed escluso dal contesto del build dell'immagine del fork.

## 11. Upload senza cancellazione (T-A007)

Confrontando le opzioni di superdufs con quelle del fork (T-A006) è emerso che upload e cancellazione si
potevano spegnere solo insieme con `--read-only`. Su richiesta dell'utente `--webui-allow-delete=false`
riproduce `--allow-upload` senza `--allow-delete`: `DELETE`, `MOVE`, `PUT` su un file non vuoto e `COPY`
su una destinazione esistente rispondono `403`, la pagina nasconde cancellazione, spostamento e modifica.
Il rifiuto vale per tutti i client e non solo per i browser, perché un divieto aggirabile con un client
WebDAV non proteggerebbe nulla; con il default `true` nulla cambia. L'esistenza dei file è controllata
anche sul backend, perché con una cache degli elenchi lunga la VFS può non conoscere un file aggiunto da
fuori. Dettagli in [uso.md](uso.md#upload-senza-cancellazione).

## 12. Release `v1.75.2-webui.3`

Su richiesta dell'utente `docker-serve-webdav/` (T-A006) e `--webui-allow-delete` (T-A007) sono stati
portati sul ramo `v1.75-webui` prendendo i percorsi del fork da `main` (`6854a0c36`); `webdav.go` e la CI
generata non cambiavano. Footprint rispetto a v1.75.2, build, test con e senza interfaccia e lint dei
percorsi del fork sono passati sul ramo prima del tag (`0ac086ea8`). Il run del tag ha pubblicato al primo
tentativo [la release](https://github.com/manprint/rclone/releases/tag/v1.75.2-webui.3), con 51 file come
la precedente, e le immagini `1.75.2-webui.3`, `1.75`, `1` e `latest`: `latest` stampa
`rclone v1.75.2-webui.3` e ha `--webui-allow-delete`. `lint` è rosso solo per i due `errcheck` upstream di
`cmd/serve/rc.go` (§9).

## Commit del fork

Dal più recente, fino alla pubblicazione della CI; la base è il commit upstream `81e7372f6`. I commit
successivi sono nel ledger dello [stato del piano](piano/001_plan-WebdavSuperUI/STATE.md) e in `git log`.

| Commit | Oggetto |
|--------|---------|
| `b4f5c514b` | task(T-A003): webui: document fork releases of upstream point releases |
| `4f9018b56` | bug(B-A001): webui: keep the assets byte-identical on Windows checkouts |
| `d18d74e02` | task(T-A003): webui: add a fork CI generated from the upstream workflows |
| `1ef5d8ec5` | task(T-A002): webui: keep private references out of the published fork |
| `67dfded8d` | task(T-A001): webui: add a justfile to build the image and the binaries with docker |
| `efcb3e528` | phase(3): revalidate phase 3 after correction V001-C1 |
| `0e09d5765` | correction(V001-C1): webui: hold the VFS during downloads served before authentication |
| `3754dd8f7` | verify(V001): complete audit; PASS WITH FINDINGS, 3 minor |
| `56897460b` | webui: clarify links with --user-from-header and --client-ca |
| `97fbecc48` | phase(4): webui: final verification of the superdufs web interface |
| `f1ec54e3e` | sub-phase(4.2): webui: document the fork and add upstream sync and build scripts |
| `96508039e` | sub-phase(4.1): webui: lock the API used by the page and add an end-to-end smoke test |
| `74d85c895` | phase(3): webui: download links and public links verified |
| `479ae15f7` | sub-phase(3.2): webui: add permanent public links for chosen paths |
| `4ecb7e9b0` | sub-phase(3.1): webui: add expiring download links usable without credentials |
| `4d1c84ccc` | phase(2): close archives, search and disk stats after review |
| `f22d899cc` | sub-phase(2.2): search by name and disk usage stats |
| `4ffadf697` | sub-phase(2.1): download folders and selections as zip |
| `6e4a719eb` | phase(1): close browser operations, editor and resume after review |
| `919063055` | sub-phase(1.3): remember media playback positions |
| `9d7d967db` | sub-phase(1.2): edit and view pages, sandbox for active files |
| `94ead84fa` | sub-phase(1.1): browser uploads, append, login and safe moves |
| `8ea53877e` | phase(0): close foundations and listing after full gates and review |
| `20ae7a36b` | sub-phase(0.2): serve the superdufs page with the VFS directory listing |
| `8cf3d0524` | sub-phase(0.1): webui scaffold, flags and vendored superdufs assets |
