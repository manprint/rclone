# rclone con l'interfaccia web di superdufs

Questo è un fork di [rclone](https://github.com/rclone/rclone), pubblicato su
https://github.com/manprint/rclone (ramo `main`), che aggiunge a `rclone serve webdav` l'interfaccia web
di superdufs (un fork di [dufs](https://github.com/sigoden/dufs)). Un browser che apre il server vede una
pagina con navigazione, upload e download di file e cartelle, modifica dei file di testo, cancellazione,
spostamento, selezione multipla, link di download diretti, link pubblici, ricerca e un player audio e
video che si riconnette da solo, con layout per desktop e per mobile.

Tutto il resto di rclone è invariato: stessi comandi, stessi flag, stesso comportamento. I client WebDAV e
la riga di comando (`curl`, `wget`, il backend `webdav` di rclone) ricevono le risposte upstream, e con
`--webui=false` il server è identico a quello upstream.

```bash
rclone serve webdav --addr 0.0.0.0:8000 myremote:
```

Le release del fork hanno la versione `vX.Y.Z-webui.N` (per esempio `rclone v1.76.0-webui.1`) e le build
locali di `build.sh` e `just` il suffisso `-webui`. Le build della CI per i push su `main` (artifact e
immagini `main` e `beta` su `ghcr.io/manprint/rclone`) hanno la versione beta di upstream seguita dal ramo,
per esempio `rclone v1.76.0-beta.10633.b4f5c514b.main`.

Le release, con i binari per tutte le piattaforme di upstream, sono su
https://github.com/manprint/rclone/releases; l'immagine dell'ultima release è
`ghcr.io/manprint/rclone:latest`, quelle delle singole release `ghcr.io/manprint/rclone:X.Y.Z-webui.N`.

## Documenti

| Documento | Contenuto |
|-----------|-----------|
| [uso.md](uso.md) | Guida d'uso: avvio, flag, interfaccia, client WebDAV, ripresa della riproduzione, ricerca, link di download e pubblici, sicurezza, limiti noti |
| [build.md](build.md) | Build locale del binario e dell'immagine Docker (`build.sh`, `just`) |
| [../docker-serve-webdav/](../docker-serve-webdav/README.md) | Docker compose pronto per `serve webdav`: `.env` con tutte le variabili, template di `rclone.conf`, guida |
| [ci-e-release.md](ci-e-release.md) | CI del fork, binari, immagini, attivazione e procedura di release |
| [sviluppo.md](sviluppo.md) | Architettura, sincronizzazione con upstream e con superdufs, test |
| [storia.md](storia.md) | Cronologia del lavoro: richiesta, analisi, piano, fasi, audit, correzioni, pubblicazione |
| [analisi/specifiche.md](analisi/specifiche.md) | La richiesta originale |
| [analisi/ANALISI_WEBUI.md](analisi/ANALISI_WEBUI.md) | L'analisi tecnica fatta prima dello sviluppo |
| [piano/001_plan-WebdavSuperUI/](piano/001_plan-WebdavSuperUI/overview.md) | Il piano in fasi ([overview](piano/001_plan-WebdavSuperUI/overview.md), `phase_0N.md`), lo [stato di esecuzione](piano/001_plan-WebdavSuperUI/STATE.md) con evidenze e revisioni, l'[audit](piano/001_plan-WebdavSuperUI/verify/index.md) |

## Sviluppo additivo: nessun conflitto con upstream

Il fork deve poter unire ogni aggiornamento di rclone senza conflitti, quindi aggiunge e non modifica:
tutto ciò che è del fork sta in file e cartelle che upstream non ha.

| Percorso | Contenuto |
|----------|-----------|
| `webui/` | Il package Go dell'interfaccia, gli asset di superdufs, gli script, `justfile` e Docker |
| `cmd/serve/webdav/webdav_webui*.go` | Il collegamento tra `serve webdav` e il package, e i suoi test |
| `.github/workflows/fork-*.yml` | La CI del fork, generata dai workflow upstream da `webui/scripts/gen-ci.py` |
| `fork-doc/` | Questa documentazione, l'analisi e il piano |
| `docker-serve-webdav/` | Docker compose, `.env` e template di `rclone.conf` per usare l'immagine con `serve webdav` |
| `cmd/serve/webdav/webdav.go` | L'unico file upstream toccato: tre righe aggiunte, marcate `// fork:webui`, nessuna riga cambiata |

Restano come upstream, tra gli altri, `go.mod` e `go.sum` (nessuna dipendenza nuova), `Makefile`,
`Dockerfile`, `.gitattributes`, i workflow in `.github/workflows/` che non iniziano con `fork-`, il
`README.md` e la cartella `docs/`.

`webui/scripts/check-footprint.sh` fallisce se il fork tocca un file fuori da questi percorsi, o se in
`webdav.go` cambia o toglie righe o ne aggiunge più di tre. Lo eseguono `webui/scripts/sync-upstream.sh`
dopo ogni merge e il job `webui` della CI a ogni push. L'unico conflitto possibile riguarda le tre righe di
`webdav.go`, se upstream cambia proprio quelle: `rerere` ricorda la risoluzione (vedi
[sviluppo.md](sviluppo.md#con-upstream-rclone)).

Le aggiunte future seguono la stessa regola: codice nuovo in `webui/` o in file nuovi `*_webui*.go`,
documentazione e piani in `fork-doc/`, nessuna modifica ai file upstream.

## Aggiornare il fork

```bash
webui/scripts/sync-upstream.sh   # merge di upstream/master in main, rigenera la CI, controlli e test
git push origin main
```

I dettagli sono in [sviluppo.md](sviluppo.md#sincronizzazione), le release in
[ci-e-release.md](ci-e-release.md#release-del-fork).
