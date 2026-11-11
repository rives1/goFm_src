# gofm — dual-pane file manager for the Windows console

A keyboard-driven, two-panel file manager for Windows Terminal and the classic console,
in the tradition of Norton Commander, Midnight Commander and Total Commander.
A single executable (`gofm.exe`, ~7 MB) with nothing to install.

![gofm screenshot](docs/screenshot.png)

**Highlights:** copy/move with the Windows copy engine and parallel small-file copies ·
browse and extract ZIP and 7z archives · flat view of a whole folder tree · network shares
(UNC) with share lists · Recycle Bin · folder comparison · type-to-jump · automatic refresh ·
free space per panel · remembers your folders.

## Download

Get **`gofm-<version>-windows.zip`** from the [latest release](https://github.com/rives1/goFm_src/releases/latest) (a `.sha256` checksum is published next to it). The zip contains `gofm.exe` (x64), this README, the changelog, the licences and the `windows-terminal` folder with the install script.

To check a download, compare its hash with the one on the release page (PowerShell):

```
Get-FileHash .\gofm-1.21-windows.zip     # the zip: see the .sha256 file
Get-FileHash .\gofm.exe                  # the program: see the release notes
```

### Antivirus warnings

Some antivirus programs — Microsoft Defender in particular, with names ending in `!ml` such as `Trojan:Win32/Bearfoos.B!ml` — flag new, unsigned Go executables by **machine-learning heuristics**, not because they recognise malware. The Go FAQ calls this "almost always a false positive". gofm 1.10 was reported this way.

What you can rely on:
- every release is built by GitHub Actions from the public source, with the compiler pinned in `go.mod`, and the build is **reproducible**: rebuilding the tagged commit with `scripts/build-release.sh` on Linux (or WSL) gives the same bytes as the published executables (the release notes list their SHA-256);
- since 1.11 the executables carry version information, a manifest that never asks for administrator rights, and an icon (since 1.13 in 16, 32, 48 and 256 pixels, so Explorer shows it sharp in every view).

If your antivirus quarantines gofm: check the hash as above, report the false positive to the vendor (Microsoft: <https://www.microsoft.com/en-us/wdsi/filesubmission>, "Incorrectly detected"), and restore the file only if the hash matches. Please do not disable your antivirus.

## Running

```
gofm.exe                        :: both panels in the current folder
gofm.exe C:\Projects D:\Backup  :: left and right panel
```

Without arguments gofm **reopens the folders you were in** last time (see *Remembered folders*).

**Windows Terminal** is recommended (full colours and Unicode). Classic `cmd.exe` and PowerShell windows work too.
To run it from anywhere, add the folder of `gofm.exe` to your `PATH` (keep `gofm.exe` in its own writable folder: its settings live next to it, see *Settings*).

### Font with icons (Cascadia Mono NF) — install it first

gofm shows an **icon** before each name (folder, archive, program, image, document, audio/video, code, system file, other). The icons come from a **Nerd Font**, a font with extra symbols: a terminal program cannot draw pictures, only characters, so the icons are characters of that font. With an ordinary font they appear as **empty boxes**.

Install **Cascadia Mono NF** once, by hand:

1. Download `CascadiaCode-<version>.zip` from <https://github.com/microsoft/cascadia-code/releases> (the asset of the latest release).
2. Open the zip, go to the `ttf` folder, right-click **`CascadiaMonoNF.ttf`** → **Install** (or *Install for all users*).
3. Use the gofm profile below, which already selects **Cascadia Mono NF**, or set that font in your own Windows Terminal profile (*Settings → profile → Appearance → Font face*).

No font at hand, or the classic console? Press **Alt+I**: gofm switches to **basic symbols** (`► ¶ ▒ ♦ ☼ ♫ § ◘ ·`) that every console font has. The choice is remembered; Alt+I again switches back.

### Windows Terminal profile (Cascadia Mono NF)

A terminal program cannot choose its font: the terminal does. The `windows-terminal` folder adds a **gofm** profile to Windows Terminal with the **Cascadia Mono NF** font (size 11), the **gofm Navy** colour scheme and an underscore cursor. If the font is not installed, Windows Terminal says so and falls back to its default font (the icons then show as boxes: install the font or press Alt+I).

```
powershell -ExecutionPolicy Bypass -File windows-terminal\install-gofm.ps1 [-AddToPath]
```

The script copies the right executable to `%LOCALAPPDATA%\Programs\gofm`, installs the profile as a Windows Terminal *fragment* and, with `-AddToPath`, adds the folder to your user PATH. It also tells you if Cascadia Mono NF is not installed (it does not install fonts). No administrator rights are needed. Restart Windows Terminal and pick **gofm** from the drop-down. If Windows blocks the script because the zip came from the Internet, right-click the zip → Properties → Unblock before extracting.

There is no ARM build since v1.19: on Windows PCs with an ARM processor (e.g. Snapdragon laptops) Windows 11 runs `gofm.exe` through its x64 emulation; Windows 10 on ARM cannot run it.

## Keys

Typing letters in a panel jumps to the first matching name, so every command
uses a function key, **Alt+letter**, **Ctrl+letter**, or **Shift+letter** (drives).
**Ctrl+F2** shows all the keys in the program, with the version of gofm in its title.

| Key | Action |
|---|---|
| ↑ ↓ PgUp PgDn Home End | move the cursor |
| → | enter folder / browse inside a .zip or .7z (does nothing on other files) |
| ← / Backspace | parent folder, or leave the archive |
| Enter / double click | open folder / open file with its program (preview, inside an archive) |
| Tab | switch panel |
| *lower-case letters* | quick search: jump to the first name starting with (or containing) what you type (case does not matter for the match); Backspace deletes a character, Esc ends it |
| Shift+*letter* | go to that drive (Shift+D → D:), returning to the last folder you used there |
| Ctrl+B | **flat view**: all files of this folder and its subfolders in one list; Ctrl+B again to go back |
| Ctrl+T | **tree view**: folder tree on the left, contents of the highlighted folder on the right; Ctrl+T again to go back (see *Tree view*) |
| Ctrl+→ (left panel) | show this location in the right panel |
| Ctrl+← (right panel) | show this location in the left panel |
| Shift+↑ / Shift+↓ | **mark while moving**: invert the mark of the item under the cursor, then move (going back unmarks) |
| Shift+PgUp / PgDn / Home / End | invert the marks of every row passed; Shift+Home and Shift+End include the first/last row |
| Space / Ins | mark/unmark; the panel's top border shows the running total of the selection, folders included recursively (Space types a space while a quick search is in progress) |
| Ctrl+A | nothing marked → mark **all** files and folders; something marked → **invert** the marks |
| Esc | end quick search, then clear the filter, then the marks |
| F2 | rename |
| F3 | preview / properties (text, archive listing, hex, folder size) |
| F4 | **edit** the file with an external editor (Notepad, or the one set in `gofm.ini`) |
| F5 | copy to the other panel's folder, **at once** — inside an archive: **extract** the marked items there |
| F6 | move to the other panel's folder, **at once** |
| F7 | new folder (accepts `a\b\c`) |
| Ctrl+K | **add a prefix** to the names of the marked files and folders, or of the item under the cursor (see *Adding a prefix*) |
| Ctrl+R | **capitalize** each word of the names (see *Capitalizing names*) |
| Ctrl+U | **swap** the two panels (the highlight stays on the same side) |
| Ctrl+O / Ctrl+I | **copy** / **cut** the marked items (or the one under the cursor) to gofm's clipboard |
| Ctrl+P | **paste** them into the folder of the active panel (see *Copy, cut and paste*) |
| Del | delete: to the **Recycle Bin** on local drives, **permanently** on network drives (see *Deleting*) |
| Shift+Del | delete **without confirmation** (still a warning for folders with subfolders) |
| F9 | compress to ZIP |
| F8 | **operations pane**: focus on the running and queued copies/moves/pastes — Pause, Cancel, Remove (see *Progress*) |
| Ctrl+W / Alt+Q | quit (F10 does nothing since 1.16); with operations running, asks first |
| Right click | mark or unmark the item under the pointer |
| F12 | **compare** the two panels by name, size and date: marks, on each side, the files missing on the other side and the **newer** copy of files on both sides (see *Comparing folders*) |
| Alt+F7 | search subfolders by name and/or content |
| Ctrl+F7 | filter the panel (`text` or `*.pdf`) |
| Alt+U | extract the whole archive under the cursor (or the marked items while browsing one) |
| Alt+L | go to a path: `C:\x`, `%USERPROFILE%`, `~`, `\\server` (share list), `\\server\share\dir` |
| Alt+V | drive list: local, network, removable (USB) and CD/DVD drives each have their own icon and label |
| Alt+E | show the same location in the other panel (either direction) |
| Alt+O | sort by name → size → date (names start A→Z, sizes and dates largest/newest first) |
| Alt+D | reverse the sort direction; the title shows e.g. `sort: name ↑` (↑ ascending, ↓ descending) |
| Alt+H | show/hide hidden and system files (and administrative shares such as `C$`) |
| Alt+R | refresh by hand (panels also refresh automatically), re-measure marked folders and **re-read `gofm.ini`** |
| Alt+I | icons: Nerd Font icons ↔ basic symbols (remembered) |
| Ctrl+F9 | next colour scheme: **navy** (default) → **dark** → **light** (remembered) |
| Ctrl+F2 | summary of all the keys |

F1 and F10 do nothing. The **key bar** at the bottom shows as many shortcuts as fit the window, operations first (F2, F3, F5 and F7 are never shown there); **click** a shortcut in the bar to run it. To save room, the bar writes Ctrl as **C** and the letter in lower case: `C+a` is Ctrl+A, `C+F7` is Ctrl+F7 (the summary, Ctrl+F2, uses the full names).

Operations act on marked items; if nothing is marked, on the item under the cursor.

When you enter a folder, a drive or an archive, or close the flat view or the tree view, the cursor goes to the **first item** (not `..`); going up with ← or Backspace puts it on the folder you came from.

### Keyboard notes

- **Shift+letter is an upper-case letter.** Terminals deliver Shift+D exactly like the character `D`, so gofm cannot tell it apart from Caps Lock. Type **lower-case** letters for the quick search (it still finds upper-case names), and keep **Caps Lock off**: with Caps Lock on, every letter switches drive.
- **Alt+letter** never conflicts with AltGr: AltGr characters (`@ # [ ]` on Italian keyboards) go to the quick search as text.

## Network shares

- `Alt+L` and type `\\server\share` (or `//server/share`) to open a share; ← from the share root shows the **list of shares** on that server, and ← again returns to the drives.
- `Alt+L` with just `\\server` lists its shares directly. Administrative shares (`C$`, `ADMIN$`) appear with Alt+H.
- gofm uses your Windows logon. If a share asks for a different user or password, connect it once with `net use \\server\share /user:DOMAIN\name` or in Explorer, then open it in gofm.

## Archives

- **→ on a .zip or .7z** opens it like a folder: navigate with →/←, mark files and folders, **F5** (or Alt+U) extracts the selection to the other panel, **F3**/Enter previews a file.
- Alt+U on an archive file extracts all of it. If the archive holds a single top-level folder it is extracted as is, otherwise into a folder named after the archive.
- ZIP archives can also be created (F9); 7z archives are read-only.
- Entries that would land outside the destination ("Zip Slip") are refused.

## Copy, move and selection

- **F5/F6 do not ask for a destination**: the destination is always the folder shown in the other panel. If the other panel shows the drive list, a share list or the inside of an archive, gofm asks you to open a folder there first. To copy under a different name, copy and then rename with F2.
- **Copying onto itself** (both panels on the same folder, or a folder into one of its own subfolders): gofm asks about each such item — *Skip* (the default), *Skip all*, *Cancel* — and copies or moves the others. If every item is skipped, nothing happens.
- **Folders with the same name** at the destination (F5 and F6): gofm first lists them and asks *Merge* (the default button) or *Cancel*. *Cancel* (or Esc) stops the **whole** operation: nothing is copied or moved.
- **Files that already exist** at the destination, also inside the merged folders: after counting the files, gofm asks about each one, showing size and date of both copies — *Skip* (the default button), *Skip all*, *Overwrite*, *Overwrite all*, *Cancel*. Every other file is copied or moved anyway. A skipped file stays where it is: in a move, the source keeps only the skipped files.
- While you mark items, the **top border of the panel** (right side, on the path line) shows ` N selected · <size> · M files `. Marked folders are measured recursively in the background and their **size column shows the total** (`…` while measuring); ` · counting…` is shown until every folder has been measured. Unmarking a folder stops its measurement.
- After a move or a delete the cursor goes to the **next item** (or the previous one at the end of the list).
- **Ctrl+→ / Ctrl+←**: the arrow points at the panel that changes, so Ctrl+→ works from the left panel and Ctrl+← from the right one. The cursor lands on the same item, and an archive being browsed is opened on the other side too.

## Tree view (Ctrl+T)

Ctrl+T replaces the left panel with the **folder tree** of the current drive, opened down to the current folder; the right panel shows the contents of the folder under the tree cursor, and follows it as soon as the cursor rests for 150 ms (scrolling quickly does not read every folder on the way). Ctrl+T again goes back to two panels, **both on the folder highlighted in the tree**.

| Key in the tree | Action |
|---|---|
| ↑ ↓ PgUp PgDn Home End | move |
| → | open the branch; on an open branch, go to its first subfolder |
| ← | close the branch; on a closed one, go to the parent folder |
| Backspace | parent folder |
| Enter / Tab | go to the right panel (Tab there comes back to the tree) |
| Shift+*letter* | show the tree of that drive |
| Alt+H | show/hide hidden folders |
| Alt+R | re-read the tree |

- Work on the files in the **right panel** as usual: rename, delete, edit, view, Ctrl+K, open subfolders (the tree follows). **Copy, move and compare (F5, F6, F12) need two panels** and are refused while the tree is shown.
- Branches show Nerd Font chevrons (closed › / open ⌄), or `+`/`-` with the basic symbols (Alt+I). A branch that turns out to have no subfolders loses its marker.
- Folders created or deleted in the right panel appear in the tree when that panel is re-read.
- The tree always reaches the current folder (since 1.21): **hidden folders on the way** (e.g. `AppData`) are listed, dimmed, even when hidden files are not shown (the other hidden folders follow Alt+H), and a path written with **8.3 short names** (`C:\PROGRA~1`) is opened under the long names.
- Ctrl+T is not available from the drive list, a share list, an archive or the flat view. The tree view is not remembered at exit (the left panel reopens on the folder of the tree).

## Copy, cut and paste (Ctrl+O, Ctrl+I, Ctrl+P)

Besides F5/F6 (always towards the other panel), files and folders can go through **gofm's own clipboard**: **Ctrl+O** copies and **Ctrl+I** cuts the marked items (or the one under the cursor); the status bar confirms it. Open the destination folder in either panel and press **Ctrl+P**: the items are copied (or moved) there, with the same questions as F5/F6 — items onto themselves, folders with the same name, existing files. Copied items can be pasted again elsewhere; cut items are pasted once. Items deleted in the meantime are left out. Ctrl+P also works in the right panel of the tree view.

The clipboard is gofm's own, not Windows' one, and is lost when gofm is closed. Ctrl+C, Ctrl+X and Ctrl+V are not used because Windows Terminal and the classic console keep Ctrl+V for pasting text: it would never reach gofm.

## Adding a prefix (Ctrl+K)

Ctrl+K asks for a text and puts it **before the names** of the marked files and folders (or of the item under the cursor when nothing is marked), e.g. `2026_` turns `report.pdf` into `2026_report.pdf`. The dialog shows a **preview** of the first new names. The field proposes the **last prefix used** since gofm was started (it is not kept after quitting). An empty text cancels; characters that names cannot contain (`\ / : * ? " < > |`) cannot be typed. An item whose new name **already exists** is not renamed: the others are, and a message lists the skipped ones. Afterwards nothing is marked. In the flat view each item stays in its own subfolder.

## Capitalizing names (Ctrl+R)

Ctrl+R puts a **capital letter at the start of every word** of the names of the marked files and folders (or of the item under the cursor). A word starts at the beginning of the name and after a space, `_` or `-`; the other letters stay as they are, and so does the extension of a file: `rapporto_IVA-q3.pdf` → `Rapporto_IVA-Q3.pdf`, `my file.JPG` → `My File.JPG`. A dialog shows the new names first (*Rename* is the default button); names that are already capitalized are left alone. An item whose new name already exists is not renamed and is listed at the end. On Windows a change of case only is a normal rename.

## Flat view (Ctrl+B)

Ctrl+B lists **every file of the current folder and of all its subfolders** in one list, with names relative to the folder (`src\deep\util.go`). Everything works on it: marking, copy, move, Recycle Bin, rename (the file stays in its subfolder), preview, compress, compare, filter, sort (e.g. by size across the whole tree), quick search (on the file name).

- **Copy and move put all files directly into the destination folder** (`src\deep\util.go` → `dest\util.go`). If two or more marked files have the **same name**, gofm asks about each extra one, showing both files with size and date: *Skip* (the default: keep the file chosen so far), *Skip all*, *Overwrite* (take this one instead), *Overwrite all*, *Cancel*. The files left out are neither copied nor moved. Then the usual questions about files already in the destination follow.
- Ctrl+B again, or moving to any other folder or drive, returns to the normal view.
- Hidden files and folders follow Alt+H. Folders, links to folders and junctions are not listed.
- The list stops at 200,000 files (the title says so). It is built with a progress window that can be cancelled.
- After an operation the tree is re-read in the background (`flat view, updating…`). Changes inside subfolders made by other programs appear with Alt+R (automatic refresh watches the top folder only).

## Folder totals and free space

The bottom border of each panel shows the **files listed** in it (count and total size, folders excluded) and the **space available** on its drive or network share, in compact form: ` 12 files 3.4 MB │ 28.5G free (11%) `. In the flat view the file part covers the whole tree. On a narrow panel the free-space part is dropped first. Free space is measured in the background when the panel changes folder and after operations (at most every 2 seconds).

## Settings (gofm.ini) and portable use

gofm is **portable**: it keeps its two files **in the same folder as `gofm.exe`**, so you can copy that folder anywhere (a USB stick, another PC) and the settings travel with it. Nothing is written to `%APPDATA%` or the registry.

| File | Written by | Content |
|---|---|---|
| `gofm.ini` | **you** (gofm creates it, fully commented, if it is missing, and never rewrites it) | your settings |
| `state.json` | gofm | folders of the panels, active panel, last folder of each drive, icon set, colour scheme |

**gofm.ini** is a plain text file: open it with Notepad, change a value, save, and press **Alt+R** in gofm (or restart it). Each setting is explained by the comments in the file. Lines are `key = value`; `#` or `;` starts a comment; every setting starts commented out, which means "use the default".

| Setting | Values (default first) | Meaning |
|---|---|---|
| `editor` | `notepad.exe`, or a path with optional arguments | the F4 editor; the `GOFM_EDITOR` environment variable, if set, wins |
| `colorScheme` | `navy`, `dark`, `light` | colours at every start (not set: the last choice made with Ctrl+F9) |
| `icons` | `nerd`, `basic` | icon set at every start (not set: the last choice made with Alt+I) |
| `showHidden` | `no`, `yes` | hidden files in both panels at start (Alt+H toggles) |
| `sortBy` | `name`, `size`, `date` | sort order of both panels at start (Alt+O, Alt+D) |
| `deleteDefault` | `delete`, `cancel` | the button Enter presses in the Del confirmations on local drives |
| `networkDelete` | `permanent`, `off` | Del on network drives: delete permanently after a confirmation, or do nothing |
| `networkDeleteDefault` | `delete`, `cancel` | the button Enter presses in the Del confirmations on network drives |

On Alt+R only the settings whose value **changed in the file** are applied, so a refresh does not undo what you changed with the keys. An unknown key or a wrong value is shown in the status bar and ignored; the other lines still apply.

**Read-only folder**: if gofm cannot write in its own folder (for example when it is copied to `C:\Program Files`), it saves nothing — no `gofm.ini` is created and `state.json` is not written — and says so in the status bar at start; every start uses the defaults (a `gofm.ini` already in that folder is still read). The install script puts gofm in `%LOCALAPPDATA%\Programs\gofm`, which is writable.

Up to 1.12 the state was kept in `%APPDATA%\gofm\state.json`; it is not imported, and that folder can be deleted. `GOFM_STATE` and `GOFM_CONFIG` set other paths for the two files (tests, special set-ups).

## Remembered folders

The folder of each panel, the active panel and the last folder used on each drive (for Shift+letter) are saved in `state.json` next to `gofm.exe` whenever you change folder, and restored at the next start. Folders given on the command line take precedence. A folder that no longer exists is replaced by its nearest existing parent; a network folder that does not answer within 3 seconds (e.g. VPN not connected) is replaced by the current folder, so start-up never hangs. An archive or flat view is remembered as its folder.

## Comparing folders (F12)

F12 compares the **files** of the two panels by **name, size and date** and marks, in each panel, what is worth copying to the other one:

| The file is… | Marked |
|---|---|
| only on this side | here |
| on both sides, modified more than 2 seconds later here | here (the **newer** copy) |
| on both sides, same date (±2 s) but a different size | on **both** sides (gofm cannot tell which one is right) |
| on both sides, same size and date (±2 s) | nowhere |

The 2-second tolerance covers FAT/exFAT drives and many NAS, which store times with that resolution; a difference of exactly one hour (daylight saving on FAT drives) counts as a difference. Folders are ignored and the content is not read. On Windows names match case-insensitively. The comparison uses what each panel lists, so the filter (Ctrl+F7) and the hidden-files setting (Alt+H) count. Previous marks are replaced, and the result is shown in the status bar until the next key — e.g. `Compare: left 2 only here, 1 newer · right 1 only here, 0 newer`. Typical use: F12, then F5 in each panel to bring the other side up to date without overwriting newer files with older ones (F5 still asks about each file it would overwrite). It also works inside archives and in the flat view (relative paths).

## Fast copying over the network

- Every file is copied with **CopyFileEx**, the Windows copy engine used by Explorer and robocopy. Windows chooses the best I/O for the medium and, where the server supports it, copies between two folders of the **same server** (SMB server-side copy, or ODX on storage arrays) without the data passing through your PC. Between **different servers** the data does go through your PC.
- Files of **256 MB or more** are copied with unbuffered I/O (like `robocopy /J`), so they do not flood the system cache.
- **Small files are copied 4 at a time** across the whole selection: each file costs a few network round trips, and running them in parallel overlaps the waits (like `robocopy /MT`). **Big files (≥ 64 MB) go one at a time**: parallel big streams do not add bandwidth on one link or disk, and slow down spinning disks.
- Attributes, dates and alternate data streams are copied; NTFS permissions are inherited from the destination folder, as with an Explorer copy. Paths longer than 260 characters are supported.
- A move renames instantly what it can (same drive) and copies the rest in one batch; each source is deleted only if all of it was copied.

**Preparing a copy (since 1.17).** Before the first byte is copied, gofm reads each source folder **once** to count the bytes and to find the files that already exist at the destination; when the same folder exists there, it reads that folder once too and compares the names in memory. The copy then uses what was read instead of reading the folders again. Before 1.17 the tree was walked three times and every file cost two separate checks — with 10 000 files already at the destination that is about 20 000 round trips to the server, i.e. minutes on a slow VPN (an estimate: it depends on the latency). A move to a free name on the same drive is a rename and is not read at all.

Marked folders are measured at most three at a time, so marking hundreds of folders on a NAS or a spinning disk does not start hundreds of competing scans.

**Large listings (since 1.20).** Sorting a folder or a flat view is more than twice as fast (200 000 names in about 60 ms), and redrawing the rows after a mark or a refresh is about 20% faster.

## Progress

**Copies, moves and pastes** (F5, F6, Ctrl+P) run in the **operations pane** at the bottom of the window: the two panels get shorter and stay usable, so you can keep browsing and start other operations. Each new copy, move or paste is **queued** and starts when the previous one has ended — one at a time, in the order you started them. Its questions (files that already exist, folders to merge, items copied onto themselves) are asked when its turn comes, so they see what the previous operations wrote. The pane shows the running operation and up to three waiting ones (the title says how many more); it closes by itself when the queue is empty, and the result is shown in the status bar (`Done: …`, `Cancelled: …`, or the list of errors).

**F8** moves the focus to the pane, and F8 or Esc goes back to the panels. ↑↓ choose a waiting operation, Tab moves to the buttons: **Pause** (it becomes *Resume*), **Cancel** (the running operation) and **Remove** (the selected waiting one); the buttons also work with the mouse. Ctrl+F2 and quitting work from the pane too. Quitting while operations are running or queued asks first: *Cancel* is the default, *Quit* cancels the running one (its half-written file is removed) and drops the others.

Compress, extract (also F5 inside an archive), delete and the other operations are not queued: they still show a progress window and run at once. Copy, move, compress and extract show a progress bar with bytes done, speed (averaged over the last 5 seconds) and an estimate of the time left. A pause stops at once, also in the middle of a big file, and Cancel still works while paused; the speed and the time left are measured again after Resume. (With a server-side copy between folders of the same server, Windows reports progress only at the end of each file, so the pause starts with the next file.) During a parallel copy the file shown is the last one started. Sizes are counted first, so very large folders spend a moment on "Counting files…". A move within the same drive is an instant rename.

## Display

- Three **colour schemes**, switched with **Ctrl+F9** and remembered: **navy** (dark blue, the default), **dark** (neutral grey) and **light**. File names are **coloured by type**: archives pink, programs red, images orchid, documents blue, audio/video green (swapped in 1.19), code and config aquamarine, system files grey, folders white (black in the light scheme). The **mark colour is reserved** (yellow; dark gold in the light scheme) and no file type comes close to it. Every scheme is checked by the tests for readable contrast.
- An **icon** before each name shows its type: one icon per colour family, plus folder, `..`, share and four kinds of drive (local, network, removable/USB, CD/DVD). The icon keeps the colour of its type **even when the item is marked** (only the name turns yellow); on the cursor row of the active panel it takes the cursor's text colour, because the type colours are unreadable on the cyan cursor. Hidden items have dimmed icons. Nerd Font icons by default, basic symbols with **Alt+I** (see *Font with icons*).
- Dates: `22/10 16:37` for the current year, `22/10/2024` for older files.
- Long names are shortened to fit the window, keeping the extension (`a really very extre….pdf`), so size and date always stay visible.
- The **status bar** follows the same rule: fixed columns on the right (size, date and time, attributes, item count) and a name column that takes the remaining space.
- The text cursor (in input fields) is a **blinking underscore**.

## Scroll bar

When a list is longer than its panel (or the tree), a **scroll bar** appears on the right border: the thumb's length is the visible part of the list, its position the place of the visible rows. **Click** on the right border, or **drag** along it, to jump to that point of the list.

## Automatic refresh

Both panels refresh by themselves when files are added, removed or changed, using Windows change notifications (they also work on network shares). Nothing is polled; bursts of changes (e.g. during a copy) are grouped, and slow folders are re-read less often. The cursor, scroll position, marks and folder totals are kept. If a server does not send notifications, Alt+R refreshes by hand.

## Behaviour worth knowing

- **Conflicts** when copying, moving or extracting: if folders with the same name already exist at the destination, gofm asks first *Merge* or *Cancel* (which stops the whole operation). Then, when copying or moving, one question per existing file (see *Copy, move and selection*); when **extracting**, a single question covers all the files: *Overwrite*, *Skip existing* or *Cancel*.
- **Moving between drives** = copy + delete the source, and the source is deleted **only if the copy fully succeeded**.
- **Deleting (Del)**: on local drives the items go to the **Recycle Bin** (Windows API; if an item cannot be recycled, e.g. a file too large, Windows shows its own warning). On **network drives** — `\\server\share` paths and mapped drives — there is no Recycle Bin, so gofm says so and **deletes permanently** after a confirmation. In both cases, if the items include **folders that contain subfolders**, a second warning lists them. **Shift+Del** skips the first confirmation (Recycle Bin on local drives, permanent deletion on network drives) but keeps the warning about folders with subfolders. The button selected in these confirmations — the one **Enter** presses — is the delete button (since 1.13; *Cancel* before); `deleteDefault` and `networkDeleteDefault` in `gofm.ini` can select *Cancel* instead, and `networkDelete = off` turns Del off on network drives. Read-only files are deleted too.
- **Editing (F4)**: Notepad by default. To use another editor set `editor` in `gofm.ini` (or the `GOFM_EDITOR` environment variable, which wins) to its path, optionally with arguments, e.g. `C:\Program Files\Notepad++\notepad++.exe` or `"C:\Program Files\Microsoft VS Code\Code.exe" --reuse-window`. gofm does not wait for the editor and starts it detached from its console, so use a windowed editor (console editors such as vim are not supported). Saved changes appear by themselves.
- ZIP names are written as UTF-8 (Explorer-compatible); old archives with OEM code-page names (e.g. CP850) are decoded.
- Symbolic links and junctions are not followed (avoids endless loops); in archives they are skipped and reported.
- Long operations run in the background with a **Cancel** button; half-written files are removed.

## Known limits

- Ctrl+K, Ctrl+O, Ctrl+P: Windows Terminal and the classic console send them as keys of their own; a few other terminals may use them themselves.
- Ctrl+I: on Windows gofm tells it apart from Tab, but in the classic console **Ctrl+Tab** arrives as Ctrl+I (it only fills the clipboard, nothing is moved). Outside Windows Ctrl+I is Tab.
- Password-protected ZIP/7z archives are not supported (a clear error is shown).
- No creation of 7z archives; no RAR or other formats.
- Content search is a plain text match: it does not find text in UTF-16 files, inside .docx/.pdf (they are compressed), or accented letters in a different case.
- 64-bit Windows x64 only (on ARM, Windows 11 runs it through x64 emulation).

## Building from source

Requires **Go 1.24 or later**. Releases are built with the toolchain pinned in `go.mod` (**go1.27.3**); with the default `GOTOOLCHAIN=auto`, an older `go` downloads it by itself.

```
git clone https://github.com/rives1/goFm_src
cd goFm_src
go build -trimpath -ldflags="-s -w" -o gofm.exe .        # on Windows
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o gofm.exe .   # from Linux/macOS
go test ./...
bash scripts/build-release.sh                            # the release package, with Windows metadata
```

A plain `go build` makes a working program without the Windows metadata (version information, manifest, icon): those come from `winres/winres.json` through [go-winres](https://github.com/tc-hib/go-winres), a build tool pinned in `go.mod` (`go tool go-winres`), which `scripts/build-release.sh` runs before compiling.

The program also builds and runs on Linux and macOS, which is how most of the tests run; drives, share lists, attributes and the Recycle Bin are Windows features.

`third_party/go4` holds two small, standard-library-only packages from go4.org (Apache 2.0) used by the 7z reader, vendored to avoid pulling in the large go4.org dependency graph (see `third_party/go4/README.md`).

`scripts/build-release.sh` builds the download package (`dist/gofm-<version>-windows.zip`, its `.sha256` and the release notes taken from `CHANGELOG.md`); it runs on Linux, macOS and Git Bash, and stops if the Go in use is not the one pinned in `go.mod`.

The GitHub Actions workflows:
- `.github/workflows/ci.yml` — on every push to a branch and on pull requests: `go mod verify`, `go vet` and the tests on Windows and Linux (with the race detector on Linux), then the package as a downloadable artifact;
- `.github/workflows/release.yml` — on a pushed tag `v<version>`: the same checks, then a **GitHub Release** with the zip and its checksum.

### Publishing a release

1. Set the version in `main.go` (`const version`); to change compiler, update the `toolchain` line of `go.mod` and `GO_VERSION` in both workflows together (a test checks it). Add its section at the top of `CHANGELOG.md` (`## <version> — <date>`) and update the README and `docs/gofm-documentazione.md`.
2. Commit, then create an **annotated** tag and push both:
   ```
   git tag -a v1.10 -m "gofm 1.10"
   git push --follow-tags
   ```
   (`--follow-tags` pushes annotated tags only.) The release appears under *Releases* a few minutes later; the workflow stops if the tag does not match the version in `main.go` or the changelog has no section for it.

## Documentation

- This README: usage.
- [`docs/gofm-documentazione.md`](docs/gofm-documentazione.md): full specification and design notes (in Italian), including a prompt to regenerate the program.
- [`CHANGELOG.md`](CHANGELOG.md): version history.
- [`docs/contesto-chat.md`](docs/contesto-chat.md): project state and compact specification for continuing the work in a new chat with an AI assistant (in Italian), updated with every version.

## Licence

gofm is released under the [MIT License](LICENSE). Copyright (c) 2026 rives.

The executable includes third-party open-source components (MIT, BSD-3-Clause, Apache-2.0 and MPL-2.0 licences); see [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
