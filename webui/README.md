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
