# Build

Come si compilano il binario e l'immagine Docker del fork in locale. I binari e le immagini
ufficiali li produce la CI del fork: vedi [ci-e-release.md](ci-e-release.md).

## Con `webui/scripts/build.sh`

```bash
webui/scripts/build.sh              # scrive ./rclone
OUT=/usr/local/bin/rclone webui/scripts/build.sh
```

Compila con `go build -ldflags "-s -X github.com/rclone/rclone/fs.Version=<git describe>-webui"`, come il
`Makefile` upstream ma con il suffisso `-webui`: `rclone version` mostra per esempio
`rclone v1.75.0-556-g96508039e-webui`, così si riconosce il binario del fork. `go build` semplice funziona
ugualmente, solo con la versione di sviluppo.

## Con `just` e Docker

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
