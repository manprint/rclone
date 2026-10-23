# CI e release del fork

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

## Attivazione

Una volta sola, sul repository GitHub del fork:

1. controlla che Actions sia abilitato (Settings → Actions → General, oppure il pulsante nella tab Actions
   che GitHub mostra in alcuni fork);
2. disattiva i workflow upstream, che nel fork salterebbero tutti i job; `winget.yml` invece non controlla
   il repository e partirebbe a ogni release. È un'impostazione del repository, i file non cambiano:

   ```bash
   for w in build.yml build_publish_docker_image.yml build_publish_docker_plugin.yml winget.yml notify.yml; do
     gh workflow disable "$w" --repo manprint/rclone
   done
   ```

3. dopo la prima pubblicazione controlla che il pacchetto `ghcr.io/manprint/rclone` sia pubblico; se non lo
   è: Package settings → Change visibility;
4. facoltativi: variabile `DOCKERHUB_USERNAME` e secret `DOCKERHUB_TOKEN` per Docker Hub e il plugin;
   secret `GPG_PRIVATE_KEY` e `GPG_PASSPHRASE` per i checksum firmati.

Stato di https://github.com/manprint/rclone al 21 ottobre 2026: Actions era già abilitato e il primo push
con i workflow del fork li ha avviati; i cinque workflow upstream sono disattivati. Docker Hub non serve e
non è configurato, quindi le immagini vanno solo su ghcr.io e il workflow del plugin non parte. Non c'è una
chiave GPG: i checksum delle release sono in chiaro. Il pacchetto `ghcr.io/manprint/rclone` è pubblico (si
scarica senza login) dalla prima pubblicazione.

Prima release: [`v1.75.2-webui.2`](https://github.com/manprint/rclone/releases/tag/v1.75.2-webui.2), dal
ramo `v1.75-webui` (point release upstream v1.75.2). Il tag `v1.75.2-webui.1` non ha prodotto una
release, perché il job `release` dipendeva ancora da `lint`, rosso sul codice upstream di v1.75.2 (vedi
[storia.md](storia.md#9-prima-release-t-a005-b-a003)); il tag e l'immagine `1.75.2-webui.1` sono stati
cancellati.

## Release del fork

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

Il job `release` parte solo se sono verdi tutti i job di build e test, quello Android e `webui`. Come
upstream, che pubblica i binari dai job di build, non aspetta `lint`: il lint usa l'ultima versione di
golangci-lint, che può segnalare nel codice di una release upstream meno recente problemi corretti dopo
su `master` (in v1.75.2, per esempio, due `errcheck` in `cmd/serve/rc.go`). Un `lint` rosso su un tag va
comunque guardato: se segnala codice del fork si corregge su `main` e si fa una nuova release.

A volte GitHub non assegna un runner
macOS e cancella il job con "The job was not acquired by Runner of type hosted": è un limite di capacità di
GitHub, non un errore del codice, ma senza quel job la release non parte. Si rilanciano i job falliti o
cancellati, insieme a quelli che ne dipendono; se `release` restasse saltato, si rilancia l'intero run:

```bash
gh run list --repo manprint/rclone --workflow fork-build.yml --limit 3   # id del run del tag
gh run rerun <id> --failed --repo manprint/rclone
gh run rerun <id> --repo manprint/rclone                                # solo se serve
```

Prima di costruire i binari di un tag, `fork-build.yml` controlla che su HEAD ci sia un solo tag, nel
formato `vX.Y.Z-webui.N`, che `VERSION` valga `vX.Y.Z` e che `vX.Y.Z` sia un tag upstream: il Makefile
prende la versione dal tag su HEAD e calcola l'ultima release (`LAST_TAG`) dal tag di `VERSION`. Il tag
del fork va quindi messo sul merge della release upstream, prima di unire il `master` successivo, che
porta `VERSION` alla versione in sviluppo. Al fork vanno spinti solo i suoi tag: un tag upstream spinto
su `origin` produrrebbe binari e immagini del fork con il numero di una release upstream.

Le point release upstream (`vX.Y.Z` con Z > 0) stanno sul ramo `vX.Y-stable`, non su `master`, quindi non
si uniscono a `main`; lo stesso vale per l'ultima release upstream quando `main` è già oltre. Per una
release del fork si crea un ramo dalla release upstream e ci si applicano le modifiche del fork, cioè la
differenza tra l'ultimo commit di `master` unito a `main` e `main`:

```bash
git fetch upstream --tags
git switch -c v1.76-webui v1.76.1
git diff --binary "$(git merge-base main upstream/master)" main | git apply --3way --index
python3 webui/scripts/gen-ci.py && git add .github/workflows
git commit -m "webui: add the web UI of serve webdav to v1.76.1"
git tag -a v1.76.1-webui.1 -m "rclone v1.76.1-webui.1"
git push origin v1.76-webui v1.76.1-webui.1
```

`gen-ci.py` rigenera i workflow da quelli della release upstream. Su un ramo così il job `webui` controlla
il footprint rispetto alla release upstream di `VERSION` invece che rispetto a `master`. Il codice del
fork viene da `main`, quindi da un `master` più recente della release: la CI del tag lo compila e lo
testa sulla release prima di pubblicare. Se `git apply` lascia conflitti (di solito le 3 righe di
`cmd/serve/webdav/webdav.go`), si risolvono come in un merge.

La differenza porta anche i [backport](sviluppo.md#backport-da-upstream) con le loro patch. Se una patch
non si applica alla release, il footprint la nomina: sul ramo della release si toglie quel backport
(`git rm` della patch e `git apply -R` della patch sui file) oppure lo si rifà sulla release.

Una correzione successiva della CI del fork si porta su un ramo così prendendo `gen-ci.py` da `main` e
rigenerando i workflow sul ramo; i workflow di `main` non si copiano, perché vengono da quelli di
`master`:

```bash
git switch v1.76-webui
git checkout main -- webui/scripts/gen-ci.py
python3 webui/scripts/gen-ci.py && git add .github/workflows
UPSTREAM_REF=v1.76.1 webui/scripts/check-footprint.sh
```
