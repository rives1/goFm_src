# gofm — contesto per proseguire in una nuova chat (versione 1.15)

> Da allegare all'inizio di una nuova chat. Contiene: stato del progetto, cose in sospeso, regole di lavoro, note tecniche sull'ambiente di sviluppo e, in fondo, le **specifiche compatte complete della 1.15**. Il dettaglio di ogni scelta, con motivazioni e storico, è in `docs/gofm-documentazione.md` nel repository.

## A. Dove sono le cose

- **Repository GitHub**: <https://github.com/rives1/goFm_src> (ramo `main`).
- **Copia locale** (PC dell'utente): `C:\Users\ammin\Documents\Claude CLI\gofm-repo\goFm_src`.
- **Cartella dei pacchetti**: `C:\Users\ammin\Documents\Claude CLI\gofm-repo\`, con `gofm-<versione>-windows.zip`, `.sha256`, `gofm-<versione>-repo.zip`, `gofm-<versione>-specifiche-compatte.md`, `gofm-<versione>-contesto-chat.md` e `microsoft-false-positive-submission.md`. Nel repository: `docs/contesto-chat.md`.
- **Stato git al 30/10/2026**:
  - 1.14 inviata dall'utente (`origin/main` = `4651871`);
  - commit `versione 1.15` con tag annotato `v1.15` (questo file è nel commit stesso).
- **SHA-256 degli eseguibili**: nelle note della Release `v1.15` su GitHub e nel file `gofm-1.15-contesto-chat.md` della cartella dei pacchetti (qui non possono stare: l'eseguibile incorpora il commit che contiene questo file).

## B. In sospeso

1. **Push** della 1.15 e controllo della Release.
2. **Prove su Windows reale** della 1.15: clic e trascinamento sulla barra di scorrimento in Windows Terminal e nella console classica; Shift+↑/↓/PgUp/PgDn/Home/End nei due terminali; copia con i due pannelli sulla stessa cartella.
3. **Prove della 1.14** ancora da fare: Ctrl+T e Ctrl+K nei terminali Windows; albero su unità di rete lente.
4. **Scelte di Claude da confermare**: (1.15) Shift+PgUp/PgDn/Home/End includono la riga d'arrivo solo se è la prima o l'ultima; nell'albero Shift+frecce sono movimenti normali. (1.14) Ctrl+T rifiutato anche dall'elenco share; albero non ricordato; nessuna ricerca digitando nell'albero. (1.13) `gofm.ini` letto anche in una cartella non scrivibile; `networkDelete` + `networkDeleteDefault`; *Merge* predefinito.
5. **Defender**: segnalazione a Microsoft (`microsoft-false-positive-submission.md`); soluzione stabile = firma del codice (serve un account dell'utente).
6. **Prove su Windows della 1.12** ancora da fare (tipi di unità, Canc in rete, F4, clic sulla barra dei tasti, decodifica di Alt+F7, Ctrl+F7, Ctrl+F2, Ctrl+F9, Ctrl+A).
7. **Domanda aperta** sull'alternativa a *Cascadia Mono NF* (va chiesto il motivo).

## C. Regole di lavoro (fissate dall'utente)

- Risposte in **italiano**, dirette e critiche; spiegare i concetti, se possibile in modo visuale.
- Agire solo con certezze condivise dagli esperti; **fare domande prima di implementare** quando qualcosa è ambiguo.
- Interfaccia del programma, README e `gofm.ini` in **inglese**.
- **A ogni nuova versione aggiornare sempre**: `const version`; CHANGELOG (testo della Release); README; `docs/gofm-documentazione.md` con test (§10), limiti (§11), storico (§12), prompt (§13); le specifiche compatte; **questo file di contesto** (regola della v1.15: `gofm-<v>-contesto-chat.md` nella cartella dei pacchetti con gli SHA-256, copia in `docs/contesto-chat.md` nel repository che per gli hash rimanda alle note della Release, perché Go incorpora il commit nell'eseguibile; inviato anche nella chat); commit `versione <x>` e tag annotato `v<x>` nella copia locale (li prepara Claude; il push lo fa l'utente); il pacchetto compilato.
- Licenza MIT, "Copyright (c) 2026 rives".

## D. Note tecniche sull'ambiente di sviluppo (per Claude)

- Il lavoro si è svolto nella shell Linux collegata al PC dell'utente, nella cartella connessa `gofm-repo` montata in `$HOME/mnt/gofm-repo`.
- **Go**: si scarica da `github.com/actions/go-versions`: l'URL esatto (con il numero di build, es. `1.27.3-39437817176`) va letto da `versions-manifest.json` del ramo `main`, perché `go.dev`, `proxy.golang.org` e `sum.golang.org` non sono raggiungibili.
  - Versione delle release: **Go 1.27.3**, fissata in `go.mod` (`toolchain go1.27.3`) e in `GO_VERSION` dei due workflow. Un test verifica che coincidano.
- **Moduli `golang.org/x/*` e `gopkg.in/*`**: proxy locale `GOPROXY=file://$HOME/gp,direct`, con `GOSUMDB=off` e `GOTOOLCHAIN=local`.
  - Il proxy si ricostruisce dalle copie GitHub (`github.com/golang/<modulo>`, `go-yaml/yaml`, `go-check/check`) con `golang.org/x/mod/zip.CreateFromDir`: per ogni voce di `go.sum` `.mod` + `.info` (raw.githubusercontent al commit; pseudo-versioni risolte con l'API GitHub; senza go.mod → `module <path>`), `.zip` solo per le voci senza `/go.mod`. Controllo finale con `go mod verify` (un `.mod` scaricato male dà *checksum mismatch*: riscaricarlo).
  - `go.mod` resta pulito: l'unico `replace` è `go4.org => ./third_party/go4`.
  - `go.sum` verifica gli hash; GitHub li controlla poi contro `sum.golang.org`.
- **go-winres** v0.3.3 è registrato come `tool` in `go.mod` e genera versione, manifest *asInvoker* e icona da `winres/winres.json`. Per farlo funzionare servono nel proxy `golang.org/x/image` v0.15.0 e `golang.org/x/text` v0.14.0.
- **Git sulla cartella connessa**: serve il permesso di cancellazione (git sostituisce i file cancellandoli). Usare `-c safe.directory=* -c maintenance.auto=false -c gc.auto=0`, l'identità `rives1 <rivesone@outlook.com>` e il trailer `Co-Authored-By: Claude …`. Il repository è nato su Windows (`core.filemode=false`), quindi gli script si lanciano con `bash`.
- **Pacchetto**: `bash scripts/build-release.sh v<x>`, da lanciare in un **clone pulito del tag**. Lo script verifica tag, CHANGELOG e compilatore; produce zip, `.sha256` e note di rilascio con compilatore, commit e SHA-256.
- **Prove dell'interfaccia**: `tmux` con `TERM=xterm-256color`, eseguibile in una cartella temporanea propria (dalla 1.13 `gofm.ini` e `state.json` nascono accanto all'eseguibile), oppure `GOFM_STATE`/`GOFM_CONFIG` puntati a file temporanei. Tutta la prova va fatta in **un solo comando**, perché la sessione `tmux` non sopravvive da una chiamata all'altra.

## E. Storico in breve

| Versione | Contenuto principale |
|---|---|
| 1.9 | icone per tipo di file (Nerd Font / simboli, Alt+I) |
| 1.10 | correzione del focus alla chiusura delle finestre; seconda conferma per cartelle con sottocartelle; riepilogo dei tasti; F12 per nome+dimensione+data; Release GitHub con pacchetto |
| 1.11 | Go 1.27.3 fissato; metadati Windows (versione, manifest, icona); build riproducibile; nota sui falsi positivi degli antivirus |
| 1.15 | barra di scorrimento cliccabile e trascinabile; copia su sé stessi saltabile (Skip, Skip all, Cancel); selezione con Shift+frecce/PgUp/PgDn/Home/End; contesto anche in `docs/contesto-chat.md` |
| 1.14 | vista ad albero (Ctrl+T) con pannello destro che segue dopo 150 ms; prefisso ai nomi (Ctrl+K) con anteprima e nomi esistenti saltati |
| 1.13 | programma portatile (`gofm.ini` e `state.json` accanto all'exe, nulla salvato se non scrivibile); `gofm.ini` commentato riletto con Alt+R; domanda Merge/Cancel per le cartelle omonime in copia, spostamento ed estrazione; Invio = cancella nelle conferme di Canc (impostabile); icona a 16/32/48/256 px |
| 1.12 | Ctrl+A, F4 editor, Canc al posto di F8, Alt+F7/Ctrl+F7/Ctrl+F2, Alt+D senso dell'ordinamento, 3 schemi di colori (Ctrl+F9), barra dei tasti per priorità e cliccabile, conflitti file per file, cancellazione definitiva in rete, tipi di unità |

---

## Specifiche compatte 1.15

Riepilogo per riprendere il lavoro in una nuova chat. Il codice completo è nel repository <https://github.com/rives1/goFm_src> (copia locale: `C:\Users\ammin\Documents\Claude CLI\gofm-repo\goFm_src`) e in `gofm-1.15-repo.zip`; la documentazione estesa (con motivazioni, storico e prompt di rigenerazione) è in `docs/gofm-documentazione.md`.

### 0. Regole di lavoro

- Risposte in **italiano**; interfaccia del programma in **inglese**.
- **A ogni nuova versione aggiornare sempre e tutte** (regola della v1.10, dettagli in §0 e §9.5 della documentazione):
  1. `const version` in `main.go`;
  2. `CHANGELOG.md` (sezione `## <versione> — <data>`: è anche il testo della Release);
  3. README (inglese);
  4. `docs/gofm-documentazione.md`, compresi test, limiti, storico e prompt di rigenerazione (§13);
  5. queste specifiche compatte (`gofm-<versione>-specifiche-compatte.md`) e il **file di contesto** (`gofm-<versione>-contesto-chat.md` nella cartella dei pacchetti, con gli SHA-256, e `docs/contesto-chat.md` nel repository, che rimanda alla Release per gli hash), inviato anche nella chat (v1.15);
  6. **pubblicazione su GitHub**: tutti i file (anche `.github/` e `.gitignore`) nella copia locale `goFm_src`, `git add -A`, commit `versione <x>`, tag annotato `v<x>` preparati da Claude; l'utente esegue `git push --follow-tags`. Il workflow `release.yml` pubblica la Release con il **pacchetto compilato** `gofm-<x>-windows.zip` e il suo `.sha256`.
- Licenza **MIT**, "Copyright (c) 2026 rives". Repository pronto per GitHub.
- Prima di implementare, chiarire con domande i punti ambigui.

### 1. Prodotto

File manager a **doppio pannello** per il terminale di Windows (Windows Terminal consigliato, funziona anche nella console classica), stile Norton/Midnight/Total Commander. **Go** (minimo 1.24; release compilate con **Go 1.27.3**, fissato in `go.mod` con `toolchain go1.27.3` e nei workflow), un solo eseguibile senza dipendenze: `gofm.exe` (x64, circa 7 MB) e `gofm-arm64.exe`. Librerie: tview v0.42, tcell v2.13.10, uniseg, bodgit/sevenzip v1.6 (7z in sola lettura), fsnotify v1.8, x/sys, x/text. Compila e gira anche su Linux e macOS, per sviluppo e test.

Avvio: `gofm [left_folder] [right_folder]`; `-h`, `--help` o `/?` stampa `gofm 1.15 — usage: …`. Senza argomenti riapre le ultime cartelle (§5.13).

### 2. Interfaccia

**Layout.** Due pannelli affiancati; sotto, la barra di stato e la barra dei tasti. Mouse attivo (clic su una riga; clic su una voce della barra dei tasti = quel tasto).

**Pannello**
- Bordo superiore: a sinistra il percorso (tagliato a sinistra con `…`) e, tra parentesi, `[flat view…, filter: …, sort: name|size|date ↑|↓, hidden shown]`; `sort:` sempre presente (v1.12), ↑ crescente, ↓ decrescente.
- Bordo superiore, a destra, in giallo: ` N selected · <dimensione> · M files ` (+ ` · counting…`).
- Bordo inferiore, a destra: ` 12 files 3.4 MB │ 28.5G free (11%) `.
  - La prima parte conta i file elencati (non le cartelle, non ricorsivo).
  - La seconda è lo spazio disponibile all'utente in forma compatta, cioè senza spazio e senza "B".
  - Se manca spazio si toglie prima la parte del disco, poi quella dei file.
- Schema navy: pannello attivo con bordo `#FFE14D`, riga del cursore su `#2AB3C8` con testo `#06122E`; pannello inattivo con bordo `#5A6E9A`, riga del cursore su `#2B3F6E` con testo bianco.

**Colonne del pannello**: marca, icona, nome, dimensione, data (separate da uno spazio).
- Marca: `*` se selezionato, altrimenti spazio.
- Icona (v1.9): una cella per tipo, vedi sotto.
- Nome: si adatta allo spazio (`larghezza − 4 − 9 − 11 − 2`), troncato con `…` mantenendo un'estensione fino a 6 celle; misura in celle con uniseg. Nessun prefisso (fino alla 1.8 le cartelle avevano `\`).
- Dimensione: 9 caratteri.
  - `UP` per `..`; per le unità il tipo `LOCAL`, `NETWORK`, `REMOVABLE`, `CD/DVD` (v1.12); `SHARE`.
  - Per una cartella `<DIR>`; se la cartella è selezionata, il suo totale ricorsivo (`…` mentre lo calcola).
- Data: 11 caratteri, `gg/mm hh:mm` se dell'anno corrente, altrimenti `gg/mm/aaaa`.
- Link seguiti da ` ~`.
- Ordine: cartelle prima; poi nome, dimensione decrescente o data decrescente.

**Schemi di colori** (v1.12, `theme.go`, tutti RGB): tre schemi, **Ctrl+F9** a rotazione, ricordato in `state.json` (`colorScheme`); `colorScheme` in `gofm.ini`, se impostato, vale a ogni avvio (v1.13).
- **navy** (principale, quello di sempre): sfondo `#0A1946`; campi, pulsanti e messaggi `#1E3A78`; testo `#D5DEEE`; barre in basso `#06122E`; etichette dei moduli `#8FD3FF`; file nascosti `#7F8FB4`; selezione `#FFE14D`.
- **dark**: sfondo `#1E1E1E`, testo `#D4D4D4`, cursore `#5FA8E8`, bordi `#707070`/`#FFE14D`, barre `#121212`; famiglie e selezione come navy.
- **light**: sfondo `#F6F8FA`, testo `#1F2328`, cartelle nere, cursore `#0969DA` con testo bianco, bordi `#808994`/`#0969DA`, selezione **`#986800`** (oro scuro: il giallo su bianco è illeggibile); famiglie scurite (`#BF3989`, `#1A7F37`, `#8250DF`, `#0550AE`, `#D80838`, `#0E7C86`, `#57606A`).
- I test verificano per **ogni** schema: famiglie ≥ 120 dal colore di selezione e ≥ 60 tra loro; contrasto WCAG ≥ 4,5:1 per i testi, ≥ 3:1 per bordi e icone. Tabella completa in §3.6 della documentazione.

**Colori per estensione** (schema navy)

| Tipo | Colore |
|---|---|
| Archivi | `#FF79C6` |
| Programmi e script | `#5AF78E` |
| Immagini | `#C792EA` |
| Documenti | `#79C0FF` |
| Audio e video | `#FF6B6B` |
| Codice, configurazione, log | `#4EE6C8` |
| File di sistema e supporto | `#9AA3B5` |
| Cartelle | bianco grassetto |
| **Selezionati** | **giallo `#FFE14D` grassetto, riservato** |

I test verificano che ogni famiglia disti almeno 120 dal giallo e almeno 60 dalle altre.

**Icone** (v1.9, `icons.go`): una per famiglia + cartella, file generico, `..`, share e quattro tipi di unità (v1.12): locale `U+F0A0`/`■`, rete `U+F0E8`/`≈`, rimovibile/USB `U+F287`/`▬`, CD/DVD `U+F192`/`○`.
- Predefinite: glifi **Nerd Font** (intervallo Font Awesome: cartella `U+F07B`, `..` `U+F062`, unità `U+F0A0`, share `U+F233`, archivi `U+F1C6`, programmi `U+F120`, immagini `U+F1C5`, documenti `U+F0F6`, audio/video `U+F144`, codice `U+F1C9`, sistema `U+F013`, altri `U+F016`). Richiedono il font **Cascadia Mono NF**, installato a mano dall'utente.
- **Alt+I**: simboli semplici della codepage 437 (`► ↑ ■ ≡ ▒ ♦ ☼ ¶ ♫ § ◘ ·`), ricordati in `state.json` (`basicIcons`); `icons = nerd|basic` in `gofm.ini` vale a ogni avvio (v1.13).
- Colore: quello della famiglia **anche se selezionato**; attenuato se nascosto; sulla riga del cursore del pannello **attivo** il colore del testo del cursore `#06122E` (sul ciano tutte le famiglie hanno contrasto < 2:1).

**Barra di stato**
- Colonne fisse a destra: `%9s │ %-16s │ %-4s │ %11s`, cioè dimensione │ data e ora │ attributi RHSA │ `N items`.
- Il nome prende lo spazio rimanente.
- Durante la ricerca digitando compare il badge ` Search: … ` (e `no match` in rosso).
- Un avviso, per esempio dopo F12, Ctrl+B, Alt+I, Ctrl+F9 o F4, sostituisce la riga fino al tasto successivo.

**Barra dei tasti** (v1.12, `keybar.go`): voci per priorità, **quante ne entrano** nella larghezza (alla prima che non entra si ferma): F4 Edit · F6 Move · Del Delete · F9 Zip · F12 Compare · Ctrl+A Select · Ctrl+T Tree · Ctrl+K Prefix · Ctrl+B Flat · Alt+F7 Find · Ctrl+F7 Filter · Alt+U Extract · Alt+V Drives · Ctrl+F2 Keys · Alt+O Sort · Alt+D Reverse · Alt+H Hidden · Ctrl+F9 Colours · Alt+I Icons · Shift+X Drive X: · Alt+L Go to. A 80 colonne 8 voci, a 120 11, tutte da 215. **Mai** F2, F3, F5, F7, F10. **Clic** su una voce = evento del tasto passato a `FM.keys`, il focus resta al pannello; Shift+X non è cliccabile.

**Riepilogo dei tasti (Ctrl+F2, v1.12)**: finestra con **tutti** i tasti (sezioni Navigation, Search and selection, Files, View, Program); righe ≤ 76 celle, altezza limitata allo schermo con scorrimento; si chiude con Esc, Invio, F2, `q`. Un test verifica che ogni voce della barra vi compaia.

**Cursore di testo**: trattino basso lampeggiante.

**Profilo Windows Terminal** (`windows-terminal/`)
- `gofm-terminal-profile.json`: fragment con **Cascadia Mono NF** 11, schema "gofm Navy" e cursore `underscore`.
- `install-gofm.ps1 [-AddToPath]`: copia l'eseguibile in `%LOCALAPPDATA%\Programs\gofm` e il fragment in `…\Windows Terminal\Fragments\gofm\`; se Cascadia Mono NF manca lo segnala (non installa font).

**Finestre di dialogo**: messaggi (Modal) e conferme; richieste di testo in un Form 76×7. Con Invio l'azione va rimandata con `go app.QueueUpdateDraw(accept)` per un problema di focus di tview. Ogni testo derivato da nomi di file va passato per `tview.Escape`.
- **Focus alla chiusura** (bug corretto nella v1.10): `Pages.RemovePage` rimette il focus sul primo elemento del layout, il pannello sinistro, che diventa attivo. `closeModal` memorizza il pannello attivo prima di `RemovePage` e poi chiama `setActive(active, true)`.

### 3. Luoghi e navigazione

- **Luoghi possibili**: elenco unità (`dir=""`), elenco share (`\\server`, con `NetShareEnum`; `C$` e `ADMIN$` solo con Alt+H), cartella (anche UNC), interno di un archivio .zip/.7z, vista piatta.
- **Cartella superiore** (calcolata con logica di stringhe): `C:\` porta all'elenco unità; `\\srv\sh\` porta all'elenco share; `\\srv` porta all'elenco unità.
- **Cambio cartella**: se la lettura fallisce, si ripristina lo stato precedente.
- **Rilettura**: il cursore resta sullo stesso elemento; se è sparito va al successivo ancora presente, altrimenti al precedente. Lo scorrimento viene mantenuto.

### 4. Tasti

| Tasto | Azione |
|---|---|
| ↑ ↓ PgUp PgDn Home End | cursore |
| → | entra in cartella, unità o share; apre .zip/.7z; su un file normale niente |
| ← | cartella superiore o uscita dall'archivio |
| Backspace | cancella un carattere della ricerca, altrimenti cartella superiore |
| Invio | apre la cartella; nell'archivio mostra l'anteprima; altrimenti apre il file con il programma associato |
| Tab | cambia pannello |
| lettere **minuscole** e altri caratteri (anche `*`) | ricerca digitando (prima per inizio del nome, poi per contenimento; azzerata dopo 1,5 s) |
| **Shift+lettera** (maiuscola A–Z) | va all'unità, nell'ultima cartella usata lì. Indistinguibile dal Blocco Maiuscole: con Blocco Maiuscole attivo ogni lettera cambia unità |
| **Ctrl+A** | nulla selezionato: seleziona tutto (file e cartelle); altrimenti **inverte** (v1.12, sostituisce `*` e Alt+A) |
| **Ctrl+B** | vista piatta sì/no (§5.10) |
| **Ctrl+T** | vista ad albero sì/no (§5.15, v1.14) |
| **Ctrl+K** | prefisso ai nomi (§5.16, v1.14); le altre Ctrl+lettera non fanno nulla (Ctrl+C non chiude) |
| Ctrl+→ (pannello sinistro) / Ctrl+← (pannello destro) | stesso luogo nel pannello opposto |
| **Shift+↑/↓** | inverte la marcatura dell'elemento sotto il cursore e si sposta (v1.15) |
| **Shift+PgUp/PgDn/Home/End** | inverte le righe attraversate, esclusa quella d'arrivo salvo prima/ultima (v1.15) |
| Spazio / Ins | seleziona o deseleziona e scende (Spazio durante la ricerca va nel testo) |
| Esc | termina la ricerca, poi toglie il filtro, poi le selezioni |
| F2 / F3 / F4 / F5 / F6 / F7 / Canc / F9 / F10 | rinomina / anteprima / **editor** (v1.12) / copia / sposta / nuova cartella / cancella / zip / esci |
| F12 | confronto delle cartelle |
| **Alt+F7** / **Ctrl+F7** | cerca / filtro (v1.12; erano Alt+S / Alt+F) |
| Alt+U / Alt+L / Alt+V | estrai / vai a percorso / elenco unità |
| Alt+E / Alt+O / **Alt+D** / Alt+H / Alt+R / Alt+I / Alt+Q | stesso luogo nell'altro pannello / tipo di ordinamento / **inverte il senso** (v1.12) / nascosti / aggiorna, rilegge `gofm.ini` (v1.13) e rimisura / icone Nerd Font ↔ simboli / esci |
| **Ctrl+F2** | riepilogo di tutti i tasti (v1.12; era Alt+F1) |
| **Ctrl+F9** | schema di colori successivo (v1.12) |
| F1, F8 | nessuna funzione (F8 cancellava fino alla v1.11) |

**Decodifica dei tasti**
- Ctrl+lettera arriva in due forme, `KeyCtrlA…Z` oppure `Key('A'…'Z')` con `ModCtrl`; Ctrl+Alt va escluso perché è AltGr.
- Alt+lettera arriva come `KeyRune` con `ModAlt`.
- Tasti funzione con Ctrl/Alt (`KeyF1…F64` con `ModCtrl`/`ModAlt`) controllati **prima** di quelli semplici; le combinazioni non assegnate non fanno nulla (Ctrl+F7 non ricade su F7).

Le operazioni agiscono sui selezionati, altrimenti sull'elemento sotto il cursore.

### 5. Funzioni

#### 5.1 Copia e spostamento (F5/F6)
- Nessuna richiesta di destinazione: si copia sempre nella cartella dell'altro pannello. Se l'altro pannello non è una cartella su disco: `The destination is the other panel:\nopen a folder on disk there first.`
- Controlli bloccanti:
  - origine e destinazione coincidono;
  - cartella copiata dentro sé stessa;
  - elemento di tipo diverso con lo stesso nome;
  - nomi doppi nella selezione (vista piatta).
- **Copia su sé stessi (v1.15)**, non più blocco: per ogni elemento con origine = destinazione o cartella che finirebbe dentro sé stessa una domanda `<motivo> (i of n)` + percorso + `It cannot be copied onto itself.` (`moved` per F6) + `Skip it and go on with the others?`: **Skip** (predefinito) / Skip all / Cancel (Esc); se non resta nulla avviso `Copy: nothing to do, every item was skipped`. Poi la domanda sulle cartelle omonime.
- **Cartelle omonime (v1.13)**: se per elementi che sono cartelle (`Lstat`) la destinazione esiste già come cartella (`sameNameFolders`), **una sola domanda** prima di iniziare (`askMerge`): `A folder with the same name already exists in` / `N folders with the same name already exist in` + destinazione + nomi (max 8, `… and N more`) + `Merge the contents? Files that exist in both are asked about one by one.` + `Cancel stops the whole operation.`; **Merge** (predefinito) / **Cancel**; Cancel o Esc annullano l'intera operazione. Vale per F5 e F6 e per l'estrazione (seguito `You will then be asked about existing files.`), dove dopo Merge resta la domanda Overwrite / Skip existing / Cancel. Estrazione completa: contano la destinazione stessa (se diversa dall'altro pannello e non vuota) e le cartelle di primo livello dell'archivio (`archiveTopFolders`); selezione nell'archivio: le cartelle selezionate.
- **File già presenti (v1.12)**: dopo `Counting files…`, fase `Checking the destination…` (`findConflicts`): ogni **file** già esistente, anche dentro le cartelle omonime (che vengono unite). Una domanda per file con dimensione e data delle due copie (`(newer)` sulla più recente di oltre 2 s): **Skip** (predefinito), Skip all, Overwrite, Overwrite all, Cancel/Esc. Tutte le domande arrivano **prima** della copia (`askSync` dal lavoro in background). I file saltati (`opCtx.skip`) restano dove sono; gli altri procedono. Negli spostamenti `removeExcept` lascia nella sorgente solo i file saltati. L'estrazione dagli archivi mantiene la domanda unica Overwrite / Skip existing / Cancel.
- **Motore di copia**:
  - un unico motore per tutta la selezione, 4 file in parallelo, un solo file ≥ 64 MB alla volta;
  - su Windows `CopyFileExW` con callback di avanzamento e annullamento (`PROGRESS_CANCEL`), `COPY_FILE_NO_BUFFERING` per file ≥ 256 MB, percorsi `\\?\`;
  - altrove copia con buffer da 1 MB.
- Lo **spostamento** rinomina subito ciò che può (stessa unità) e copia il resto; l'origine viene eliminata solo per gli elementi copiati completamente.
- Avanzamento: finestra ` Working ` con barra da 60 caratteri, percentuale, `X of Y · velocità/s · m:ss left` (velocità media sugli ultimi 5 s) e pulsante Cancel.

#### 5.2 Altre operazioni
- **F2**: rinomina; non accetta `\ / . ..` né `: * ? " < > |`; in vista piatta il file resta nella sua sottocartella.
- **F7**: crea la cartella con `MkdirAll`.
- **F4 (v1.12)**: editor esterno sul file sotto il cursore: `GOFM_EDITOR`, altrimenti `editor` di `gofm.ini` (v1.13), altrimenti `notepad.exe` (programma, anche tra virgolette o con spazi se esiste, più argomenti); avviato con `DETACHED_PROCESS`, senza attenderlo; solo editor con finestra. Su cartelle e dentro gli archivi un messaggio.
- **Canc** (F8 non cancella più, v1.12). Unità **locali**: conferma `Move … to the Recycle Bin?` Yes/No. Unità **di rete** (UNC o unità mappate, `isNetworkLocation`): `Delete … PERMANENTLY?` con Delete / Cancel, poi cancellazione definitiva (`removeTree`: `RemoveAll`, e se fallisce toglie la sola lettura e riprova). **Seconda conferma (v1.10)** se tra gli elementi ci sono cartelle vere (no link) con almeno una sottocartella: controllo in background (lettura a blocchi di 256, stop alla prima sottocartella), avviso `WARNING: this folder contains subfolders:` / `WARNING: N folders contain subfolders:` con i **soli nomi** (max 8, poi `… and N more`), pulsanti `Move to Recycle Bin` (in rete `Delete permanently`) e `Cancel`. **Pulsante predefinito (v1.13)**: in tutte le conferme di Canc il pulsante che cancella è il primo ed è selezionato (Invio cancella); `deleteDefault` (locale) e `networkDeleteDefault` (rete) in `gofm.ini` possono scegliere `cancel` (`confirmDefault` + `Modal.SetFocus`); `networkDelete = off` disattiva Canc sulle unità di rete (messaggio `Deleting on network drives is turned off`). Fino alla v1.12 il predefinito era Cancel in rete e nella seconda conferma. Cestino: `SHFileOperationW`, flag ALLOWUNDO, NOCONFIRMATION, SILENT, NOERRORUI e WANTNUKEWARNING.
- **F9**: crea uno ZIP (Deflate, nomi in UTF-8) nella cartella dell'altro pannello.

#### 5.3 Archivi
- → apre .zip/.7z come una cartella; F5 o Alt+U estraggono la selezione nell'altro pannello.
- Alt+U su un archivio estrae tutto; se l'archivio ha un'unica cartella radice non crea una cartella doppia.
- Protezione Zip Slip; i link vengono saltati.
- Archivi cifrati: `archive is password-protected (not supported)`.
- Nomi ZIP in codepage OEM decodificati con `GetOEMCP`.

#### 5.4 Ricerca (Alt+F7) e filtro (Ctrl+F7)
- Ricerca: nome con caratteri jolly e/o testo contenuto (a blocchi di 256 KB), massimo 2000 risultati.
- Filtro: `testo` oppure `*.ext`.

#### 5.5 Anteprima (F3)
- Testo fino a 512 KB: UTF-8, UTF-16 con BOM, altrimenti Windows-1252.
- File binari: dump esadecimale.
- Archivi: elenco del contenuto.
- Cartelle: dimensione calcolata in modo asincrono.

#### 5.6 Confronto (F12) — v1.10
- Solo **file** elencati, per **nome** (senza maiuscole su Windows), **dimensione** e **data** con tolleranza **2 s** (FAT/exFAT/NAS); 1 ora esatta = differenza; data zero = mai più recente.
- In ogni pannello sostituisce la selezione con: i file **mancanti** nell'altro; per i file presenti in entrambi la copia **più recente** (anche con dimensione diversa); stessa data ±2 s ma dimensione diversa → **entrambe**.
- Avviso: `Compare: left N only here, M newer · right P only here, Q newer` (+ ` · K same date, different size`), oppure `Compare: same files on both sides (name, size, date ±2 s)`.
- Funzione pura `compareSides(left, right, foldCase) compareResult`.

#### 5.7 Selezione
- Totale cumulativo sul bordo superiore.
- Le cartelle selezionate sono misurate in background e la misura si annulla se le deselezioni.

#### 5.8 Aggiornamento automatico
- Usa fsnotify (`ReadDirectoryChangesW`) sulle cartelle di entrambi i pannelli.
- Gli eventi vengono raggruppati: attesa di `max(400 ms, 2 × durata dell'ultima lettura)`.
- Cursore, selezioni e totali vengono mantenuti. Se le notifiche non sono disponibili resta Alt+R.

#### 5.9 UNC
- Alt+L accetta `\\server` (elenco share) e `\\server\share\…`.
- In caso di errore suggerisce di collegare la share con `net use`.

#### 5.10 Vista piatta (Ctrl+B)
- Funziona solo in cartelle su disco.
- Si costruisce in una finestra annullabile e mostra solo i file, con il percorso relativo (`src\deep\util.go`).
- Link e junction non vengono seguiti; i nascosti seguono Alt+H; massimo 200 000 file.
- Ctrl+B di nuovo, oppure ogni cambio di cartella, torna alla vista normale.
- F5/F6 mettono tutti i file direttamente nella destinazione e rifiutano i nomi doppi elencandoli.
- F12 confronta i percorsi relativi; la ricerca digitando confronta il nome del file.
- Dopo un'operazione la lista si rilegge in background (`updating…`).
- Le notifiche osservano solo la cartella radice: le modifiche nelle sottocartelle compaiono con Alt+R.

#### 5.11 Spazio su disco
- `GetDiskFreeSpaceExW` (percorso con `\` finale) oppure `statfs`.
- Misura asincrona, al massimo ogni 2 s per cartella.

#### 5.12 Ctrl+→ / Ctrl+←
La freccia indica il pannello che cambia; nella direzione opposta non succede nulla.

#### 5.12b Tipi di unità (v1.12)
- `GetDriveType`: rete, rimovibile, CD/DVD; i dischi **fissi** su bus USB/1394/SD/MMC (letto con `DeviceIoControl(IOCTL_STORAGE_QUERY_PROPERTY)` su `\\.\X:` aperto con accesso 0) sono **rimovibili**; il resto locale.

#### 5.12c Ordinamento (v1.12)
- `..` in cima, cartelle prima, poi per tipo nel senso del pannello; a parità per nome A→Z. Alt+O cambia tipo e riparte dal senso naturale (nome ↑, dimensione e data ↓); Alt+D inverte. Il senso non viene salvato.

#### 5.13 Memoria delle cartelle
- File (v1.13): `state.json` **nella cartella dell'eseguibile** (sostituibile con `GOFM_STATE`); fino alla v1.12 `%APPDATA%\gofm\state.json`, **non importato**. Mai scritto se la cartella non è scrivibile.
- Contenuto: `{version, left, right, active, driveDirs, basicIcons, colorScheme}`.
- Salvataggio atomico a ogni cambio di cartella, a ogni Alt+I e Ctrl+F9, solo se qualcosa è cambiato.
- All'avvio:
  - una cartella che non risponde entro 3 s diventa la cartella corrente;
  - una cartella locale che non esiste più diventa la prima cartella superiore esistente;
  - gli argomenti della riga di comando hanno la precedenza.

##### 5.14 Programma portatile e `gofm.ini` (v1.13, `config.go`)
- **Posizione**: `appDir()` = cartella di `os.Executable()` dopo `EvalSymlinks`; `appDirInfo` (`sync.OnceValues`) calcola una volta cartella e scrivibilità (`dirWritable`: crea e cancella `.gofm-write-test-*`). `findStorage()`: `state.json` / `GOFM_STATE`, `gofm.ini` / `GOFM_CONFIG`.
- **Cartella non scrivibile**: niente stato né impostazioni salvati, `gofm.ini` non creato, file già presenti letti; avviso `The gofm folder is read-only: settings and folders are not saved`. Ogni avvio riparte dai predefiniti.
- **`gofm.ini`**: scritto dall'utente, mai riscritto da gofm; se manca (e si può scrivere) viene creato **tutto commentato**, CRLF, con un blocco di commenti per ogni chiave. Sintassi `chiave = valore`, commenti `#`/`;`, `[sezioni]` ignorate, BOM tolto, chiavi e valori senza maiuscole, virgolette facoltative, `chiave =` = non impostata. Errori (`line N: …`) ignorati riga per riga e mostrati nella barra di stato.
- **Chiavi** (predefinito per primo): `editor` (`notepad.exe`; `GOFM_EDITOR` prevale), `colorScheme` navy|dark|light, `icons` nerd|basic (non impostate = ultimo valore di `state.json`), `showHidden` no|yes, `sortBy` name|size|date (all'avvio nei due pannelli), `deleteDefault` delete|cancel, `networkDelete` permanent|off, `networkDeleteDefault` delete|cancel.
- **Alt+R** (`reloadSettings`): rilegge (ricrea se manca) e applica **solo le chiavi cambiate nel file**, così non annulla Alt+H, Alt+O, Ctrl+F9, Alt+I; avviso `Settings reloaded from gofm.ini`.
- Lo script di installazione copia solo `gofm.exe` in `%LOCALAPPDATA%\Programs\gofm`: reinstallando, `gofm.ini` e `state.json` restano.


##### 5.15 Vista ad albero (Ctrl+T, v1.14, `tree.go`)
- Rifiutata (finestra `Tree`) da archivio, vista piatta, elenco unità ed elenco share.
- L'albero dell'unità (`treeRoot`: `C:\`, `\\srv\share\`, `/`) prende il posto del pannello sinistro, aperto fino alla cartella del pannello attivo; il pannello destro mostra la cartella sotto il cursore **150 ms** dopo che il cursore si ferma (`treeDelay`, timer con contatore di generazione; errore solo nella barra di stato).
- Righe: rientro di 2 spazi per livello, marcatore `U+F054`/`U+F078` (con Alt+I `+`/`-`; nessuno se aperta e senza sottocartelle), icona cartella, nome; figli letti alla prima apertura, solo cartelle, nascoste con Alt+H (flag del pannello sinistro); titolo ` Tree  <radice> `.
- Tasti sull'albero: frecce/PgUp/PgDn/Home/End; → apre o va alla prima sottocartella; ← chiude o va al padre; Backspace padre; Invio/Tab al pannello destro (subito); Shift+lettera altra unità; Alt+H, Alt+R (rilettura), Alt+I, Ctrl+F9, Ctrl+F2, F10, Alt+Q; gli altri tasti solo l'avviso `Tree: Tab or Enter moves to the files on the right · Ctrl+T back to two panels`.
- A destra tutto normale salvo F5, F6, F12 ed estrazione della selezione: `This needs the two panels:\nleave the tree view with Ctrl+T first.`. L'albero segue il pannello destro (cambio di cartella → `reveal`; rilettura → ramo riletto); Alt+E/Ctrl+← rivelano la sua cartella.
- Ctrl+T di nuovo: due pannelli, **entrambi sulla cartella evidenziata**. Modalità non salvata; `state.json` registra a sinistra la cartella dell'albero.

##### 5.16 Prefisso ai nomi (Ctrl+K, v1.14, `prefix.go`)
- Elementi: selezionati (file e cartelle) o quello sotto il cursore; fuori da una cartella su disco i messaggi di sola lettura.
- Finestra 76×14 ` Add a prefix to … `, campo `Prefix: ` con caratteri non validi non digitabili (`\ /`, su Windows anche `: * ? " < > |`), anteprima `<nome> → <prefisso><nome>` (6 righe, poi `… and N more`); vuoto = annulla.
- In background: rinomina nella stessa cartella (vista piatta: la sottocartella resta); nome già esistente = **saltato**. Dopo: nessuna selezione; se saltati, messaggio `Renamed N of M items.` + elenco (max 8), altrimenti avviso `Renamed N items`.


##### 5.17 Barra di scorrimento (v1.15, `scrollbar.go`)
- Sul bordo destro di pannelli e albero, solo se l'elenco non entra: `█` nel colore del bordo, lunghezza `max(1, round(h²/n))`, inizio `round(offset·(h−lung.)/(n−h))`.
- Tasto sinistro premuto sul bordo destro → trascinamento; il cursore va alla riga `round(y·(n−1)/(h−1))`; si aggiorna muovendo con il tasto premuto; il pannello prende il focus.

### 6. Codice

| File | Contenuto |
|---|---|
| main.go | FM, tasti, finestre, runTask/avanzamento, azioni, confronto, expandPath |
| panel.go | Panel: luoghi, caricamento, rendering, bordi, navigazione, selezione, nextSurvivor, spazio su disco |
| theme.go | tre schemi di colori, `applyScheme`, famiglie per estensione (colore + tipo di icona) |
| icons.go | icone: tipi, glifi Nerd Font, simboli semplici, colore |
| help.go | riepilogo di tutti i tasti (Ctrl+F2) |
| keybar.go | barra dei tasti: priorità, larghezza, clic del mouse |
| editor.go | editor esterno (F4) |
| drives.go | tipi di unità, `isNetworkLocation` |
| statusbar.go | barra di stato |
| copy.go | motore di copia parallelo |
| ops.go | opCtx, percorsi, zip, ricerca |
| archive.go | ZIP/7z, estrazione, albero degli archivi |
| preview.go | anteprima |
| unc.go | percorsi Windows/UNC |
| flat.go | vista piatta |
| state.go | memoria delle cartelle |
| config.go | file accanto all'eseguibile, scrivibilità, `gofm.ini` (v1.13) |
| tree.go | vista ad albero (v1.14) |
| prefix.go | prefisso ai nomi (v1.14) |
| scrollbar.go | barra di scorrimento e Shift+movimento (v1.15) |
| watch.go | aggiornamento automatico |
| sys_windows.go | Win32: unità, attributi, ShellExecute, Cestino, OEM CP, NetShareEnum, CopyFileExW, spazio su disco |
| sys_other.go | equivalenti per Linux/macOS |
| test | `*_test.go` (`v19_test.go` icone, `v110_test.go` confronto, F8, riepilogo, focus), fixture in `testdata/*.7z` |
| scripts/build-release.sh | pacchetto scaricabile: risorse Windows, zip, `.sha256`, note dal CHANGELOG |
| winres/ | `winres.json`, `icon.png` (256), `icon48.png`, `icon32.png` (v1.13), `icon16.png` (metadati Windows) |
| scripts/make-small-icons.py | icone a 32 e 48 px disegnate a pixel (Pillow) |
| v111_test.go | compilatore fissato, risorse Windows |
| v112_test.go | schemi, unità, ordinamento, tasti, barra e clic, editor, conflitti, cancellazione definitiva |
| v115_test.go | barra (calcolo, schermo simulato, clic/trascinamento, albero), Shift+movimento, copia su sé stessi |
| v114_test.go | albero (apertura, chiusura, nascosti, rifiuti, F5/F6/F12, albero che segue, uscita, ritardo), prefisso (anteprima, nomi saltati, vista piatta, caratteri non digitabili) |
| v113_test.go | `gofm.ini`, file accanto all'eseguibile, sola lettura, pulsanti di Canc, Merge/Cancel, icone 16/32/48/256 (`TestMain` imposta `GOFM_CONFIG` temporaneo) |

`third_party/go4`: `syncutil` e `readerutil` copiati da go4.org (Apache 2.0), con `replace go4.org => ./third_party/go4`.

### 7. Repository e compilazione

- Repository <https://github.com/rives1/goFm_src> (ramo `main`): `LICENSE`, `README.md`, `CHANGELOG.md`, `THIRD_PARTY_NOTICES.md`, `.gitignore`, `docs/`, `scripts/build-release.sh` e due workflow:
  - `.github/workflows/ci.yml`: sui rami e le pull request; verify, vet e test su Windows e Linux, race detector su Linux, pacchetto come artifact;
  - `.github/workflows/release.yml`: al push di un tag `v<versione>`; verifiche, `bash scripts/build-release.sh "$GITHUB_REF_NAME"` (si ferma se il tag non corrisponde a `main.go` o manca la sezione nel CHANGELOG), `gh release create` con `gofm-<v>-windows.zip` e `.sha256`.
- Pacchetto: cartella `gofm-<v>/` con `gofm.exe`, `gofm-arm64.exe`, README, CHANGELOG, LICENSE, THIRD_PARTY_NOTICES, `windows-terminal/`.
- Il repository è nato su Windows (`core.filemode=false`): gli script si lanciano con `bash`.
- **Compilatore (v1.11)**: Go 1.27.3 in `GO_VERSION` dei due workflow (con `GOTOOLCHAIN=local`) e nella riga `toolchain` di `go.mod`; lo script si ferma se `go env GOVERSION` è diverso; un test verifica che workflow e `go.mod` coincidano. Fino alla 1.10 `setup-go` leggeva la riga `go 1.24.0` e usava Go 1.24.0.
- **Metadati Windows (v1.11)**: `go tool go-winres make --arch amd64,arm64 --product-version <v> --file-version <v>` da `winres/winres.json` (go-winres v0.3.3, 0BSD, registrato come `tool` in `go.mod`). Versione (ProductName gofm, CompanyName rives, FileDescription, LegalCopyright, OriginalFilename gofm.exe, Comments con il link), manifest `asInvoker` + `longPathAware`, icona originale a due pannelli (256, 48, 32 e 16 px; 32 e 48 dalla v1.13). I `.syso` sono generati dallo script e ignorati da git. Build deterministica (verificato).
- **Note di rilascio**: compilatore, commit e SHA-256 dei due eseguibili. **Build riproducibile**: per la 1.10 verificato che la ricompilazione del commit dia gli stessi byte della Release; per la 1.13 due compilazioni dal tag danno gli stessi byte.
- **Antivirus**: la 1.10 è stata segnalata da Defender come `Trojan:Win32/Bearfoos.B!ml` (euristica ML, falso positivo tipico dei binari Go non firmati). Testo per Microsoft in `microsoft-false-positive-submission.md`. Soluzione stabile non ancora attiva: **firma del codice** (SignPath Foundation o Azure Trusted Signing, serve un account dell'utente).
- `go.mod` è standard. Il `go.sum` è stato costruito da mirror GitHub con un metodo verificato, perché `golang.org` e `sum.golang.org` non erano raggiungibili.
- Compilazione: `GOOS=windows GOARCH=amd64|arm64 go build -trimpath -ldflags="-s -w"`.
- In un ambiente senza accesso a `golang.org` servono i `replace` verso `github.com/golang/*` oppure un proxy locale (`GOPROXY=file://…`, `GONOPROXY=github.com`).
- Il nome del modulo è `gofm`; per permettere `go install` va cambiato in `github.com/<utente>/gofm`.

### 8. Limiti e parti non verificate

**Limiti**
- F4: solo editor con finestra (avviati staccati dalla console).
- Eseguibili non firmati: possibili falsi positivi degli antivirus.
- Icone: senza un Nerd Font i glifi sono quadratini e il programma non può accorgersene (serve Alt+I); un'icona per famiglia, non per estensione.
- Nessun supporto agli archivi con password né alla creazione di 7z.
- Ricerca per contenuto solo testuale.
- Solo Windows a 64 bit.
- Ricerca digitando solo con le minuscole; Blocco Maiuscole sconsigliato.
- Vista piatta: modifiche nelle sottocartelle visibili solo con Alt+R.
- Albero: rami letti all'apertura, aggiornati solo con Alt+R o quando il pannello destro rilegge quella cartella; per copiare o spostare si esce dall'albero.
- Cartella del programma non scrivibile: nulla viene salvato. Con Alt+R, commentare un valore di `gofm.ini` non ripristina il predefinito fino al riavvio.

**Non verificato su Windows reale** (solo compilato e testato su Linux):
- Cestino, NetShareEnum, CopyFileExW;
- spazio su disco UNC;
- codifica dei tasti nei vari terminali, resa nella console classica;
- script PowerShell di installazione;
- icone Nerd Font a schermo (glifi e larghezze verificati sul file del font);
- icona e proprietà del file in Esplora risorse; effetto dei metadati su Defender (prima verifica con la Release 1.11);
- v1.12: tipi di unità e bus USB, cancellazione su unità di rete, F4 con Notepad, clic sulla barra, decodifica di Alt+F7, Ctrl+F7, Ctrl+F2, Ctrl+F9 e Ctrl+A nei terminali Windows (nella console classica Ctrl+A potrebbe essere intercettato dalla selezione del testo).

- v1.15: mouse sulla barra (clic e trascinamento) e Shift+frecce/PgUp/PgDn/Home/End nei terminali Windows.
- v1.14: Ctrl+T e Ctrl+K nei terminali Windows; albero su share lente (apertura dei rami sincrona).
- v1.13: cartella in sola lettura reale (`C:\Program Files`), avvio da collegamento o PATH, `gofm.ini` salvato da Notepad, icone a 32/48 px in Esplora risorse (cache delle icone).

La CI di GitHub su `windows-latest` sarà la prima verifica.
