# Richiesta originale

Testo con cui è stato chiesto il fork, il 21 ottobre 2026, riportato senza modifiche. Nella stessa
conversazione sono arrivati due vincoli: mantenere tutte le opzioni e i flag di rclone ("noi dobbiamo solo
aggiungere, non modificare l'esistente") e poter sempre unire gli aggiornamenti upstream senza conflitti.

```text
Vorrei fare un fork di rclone, per poternziare la componente `webdav serve` con un nuovo frontend.
Il nuovo frontend, vorrei che fosse come quello di `superdufs`.
Il nuovo frontend di rclone serve webdav dovrebbe mostrare tutte le opzioni e le funzionalità di quello di superdufs:
Alcune di queste funzionalità sono:
- possibilità di caricare cartelle / scaricare cartelle
- possibilità di caricare file / scaricare file
- Possibilità di editare file
- possibilità di cancellare cartelle / file
- Possibilità di spostare cartelle e file
- Selezione multipla
- Possibilità di generare link diretti di dowload er file cartelle
- Resilient player per video ed audio come in superdufs
- Visualizzazione mobile/desktop come in superdufs

Insomma, al lancio del comando:
rclone serve webdav --addr 0.0.0.0:8000 myremote:
Vorrei trovarmi con l'interfaccia di superdufs e poterne sfruttare tutte le opzioni disponibili.
Mi fai un analisi approfondita?
Nota bene: voglio sempre poter aggiornare il fork dall'upstream, quindi dobbiamo fare in modo che i merge non diano problemi.
```
