# Changelog

All notable changes to gofm. Versions follow the order in which features were added.

## 1.15 — 2026-10-30

- **Scroll bar** on the right border of the panels and of the tree, only when the list is longer than the box; click or drag on it to jump to that point of the list.
- **Copying onto itself** (same folder on both sides, or a folder into one of its subfolders) no longer stops the whole operation: one question per item, *Skip* (default), *Skip all*, *Cancel*; the other items are copied or moved.
- **Shift+↑/↓** mark while moving (the item under the cursor is inverted, then the cursor moves); **Shift+PgUp/PgDn/Home/End** invert every row passed, Shift+Home/End including the first/last row.
- The context summary for continuing in a new chat is also kept in the repository: `docs/contesto-chat.md`.

## 1.14 — 2026-10-30

- **Tree view (Ctrl+T)**: the left panel becomes the folder tree of the current drive, opened down to the current folder; the right panel shows the folder under the tree cursor, 150 ms after the cursor stops. → and ← open and close branches, Enter or Tab go to the files on the right, Shift+letter shows another drive, Alt+H shows hidden folders. Ctrl+T again goes back to two panels, both on the highlighted folder. Copy, move and compare need two panels and are refused meanwhile; not available from the drive list, share lists, archives or the flat view.
- **Add a prefix (Ctrl+K)**: puts a text before the names of the marked files and folders (or of the item under the cursor), with a preview of the new names. An empty text cancels; invalid characters cannot be typed; items whose new name already exists are skipped and listed. Nothing stays marked afterwards.
- Key bar and key summary (Ctrl+F2) list Ctrl+T and Ctrl+K.

## 1.13 — 2026-10-30

- **Portable**: the settings and the remembered state now live **next to `gofm.exe`** (`gofm.ini`, `state.json`) instead of `%APPDATA%\gofm`, so the program folder can be copied anywhere with its settings. If that folder is not writable (e.g. `C:\Program Files`) gofm saves nothing and says so at start. The old `%APPDATA%\gofm\state.json` is not imported.
- **Settings file `gofm.ini`**: a commented text file, created at the first start, never rewritten by gofm. Settings: `editor` (F4), `colorScheme`, `icons`, `showHidden`, `sortBy`, `deleteDefault`, `networkDelete` (`permanent` or `off`), `networkDeleteDefault`. Read at start and again with **Alt+R**, which applies only the values changed in the file. `GOFM_EDITOR` still wins over `editor`.
- **Folders with the same name** at the destination of a copy, move (F5/F6) or extraction: gofm lists them and asks *Merge* or *Cancel*; *Cancel* stops the whole operation. After *Merge* the questions about existing files follow as before.
- **Delete confirmations**: the button selected at first (Enter) is now the delete button, also for permanent deletion on network drives and for the warning about folders with subfolders; `gofm.ini` can set *Cancel* instead, separately for local and network drives.
- **Icon**: added 32 and 48 pixel versions, drawn pixel by pixel, used by Explorer in the medium and large views (before, only 16 and 256 pixels).

## 1.12 — 2026-10-30

- **Keys**: Ctrl+A marks everything, or inverts the marks (`*` and Alt+A removed); **F4** edits the file with an external editor (Notepad, or the one in `GOFM_EDITOR`); **F8 no longer deletes** (Del only); find moves to **Alt+F7**, filter to **Ctrl+F7**, the key summary to **Ctrl+F2**, now listing every key; **Alt+D** reverses the sort, with an arrow in the title (`sort: name ↑`); **Ctrl+F9** cycles three colour schemes.
- **Key bar**: shows as many shortcuts as fit the window, operations first; F2, F3, F5, F7 and F10 are no longer shown; **click** a shortcut to run it.
- **Colour schemes**: navy (the default, unchanged), dark and light, remembered in `state.json`; every scheme is tested for readable contrast.
- **Copy/move conflicts**: one question per existing file, also inside merged folders, showing both copies: Skip (default), Skip all, Overwrite, Overwrite all, Cancel. The other files are copied or moved anyway; skipped files stay where they are. (Before, a single question covered only the top-level items, and files inside merged folders were overwritten.)
- **Network drives**: Del deletes permanently, with an explicit warning, instead of going through the Recycle Bin (network folders have none). Read-only files are deleted too.
- **Drive list**: local, network, removable (USB sticks and disks, memory cards) and CD/DVD drives have their own icon and label.

## 1.11 — 2026-10-28

- **Compiler updated**: releases are built with **Go 1.27.3**, pinned in `go.mod` (`toolchain go1.27.3`) and in both workflows. 1.10 had been built with Go 1.24.0, because the workflows took the minimum version from `go.mod`; the Go 1.24 series no longer receives security fixes. A test keeps the workflows and `go.mod` in step, and the build script refuses any other compiler.
- **Windows metadata** in `gofm.exe` and `gofm-arm64.exe`: version information (product, version, description, copyright, link to the source), an application manifest (*asInvoker*: never asks for administrator rights; long-path aware) and an icon. Made by go-winres (0BSD), pinned as a `tool` in `go.mod`, from `winres/winres.json`. Unsigned executables without metadata are more often reported by antivirus heuristics.
- The release notes list the compiler, the commit and the SHA-256 of both executables; the README explains how to check a download and what to do about antivirus warnings.
- 1.10 (built from the same kind of source) was reported by Microsoft Defender as `Trojan:Win32/Bearfoos.B!ml`, a machine-learning detection that also hits other Go programs. The published 1.10 executables were rebuilt byte for byte from the public commit: they contain nothing but the source code.

## 1.10 — 2026-10-28

- **Fix**: closing any dialog (progress, confirmation, message, preview) opened from the right panel moved the focus to the left panel — most visible with Ctrl+B. The active panel is now kept.
- **F8 / Del**: when the items include folders that contain subfolders, a second confirmation lists them; its default button is *Cancel*.
- **Alt+F1**: summary of the shortcuts that are not in the key bar.
- **F12 compares name, size and date** (±2 s, the resolution of FAT/exFAT and many NAS). On each side it marks the files missing on the other side and, for files on both sides, the **newer** copy; same date but different size marks both. F5 after F12 copies only what is new or newer.
- **GitHub releases**: pushing a tag `v<version>` builds the Windows package and publishes it as a Release with a SHA-256 checksum (`.github/workflows/release.yml`, `scripts/build-release.sh`). The CI builds the same package as an artifact.
- The 1.9 files missing from the repository (documentation, Windows Terminal profile, install script, CI workflow, `.gitignore`) are included.

## 1.9 — 2026-10-28

- **File-type icons** before each name: one per colour family (archive, program, image, document, audio/video, code, system), plus folder, plain file, `..`, drive and share. Folders lose the `\` prefix.
- Icons are **Nerd Font** glyphs (needs a font such as *Cascadia Mono NF*); **Alt+I** switches to basic symbols that every console font has. The choice is saved in `state.json` (`basicIcons`).
- The icon keeps the family colour when the item is marked; on the cursor row of the active panel it takes the cursor text colour (family colours are unreadable on the cyan cursor).
- The Windows Terminal profile uses **Cascadia Mono NF**; the install script warns if the font is missing. The README explains how to install it.
- The mark (`*`) and the icon are separate columns: the name column is 2 cells narrower.

## 1.8 — 2026-10-27

- Drives switch with **Shift+letter** (an upper-case letter) instead of Ctrl+letter; the quick search uses lower-case letters. With Caps Lock on, letters switch drive.
- Flat view moves from Alt+D to **Ctrl+B**.
- The bottom border also shows the files listed in the panel and their size, and the free space in compact form: ` 12 files 3.4 MB │ 28.5G free (11%) `.

## 1.7 — 2026-10-27

- **Remembered folders**: both panels, the active panel and the last folder per drive are saved in `%APPDATA%\gofm\state.json` and restored at start-up (command-line folders win; missing folders fall back to their nearest parent, unresponsive network folders to the current folder after 3 s).
- **Free space** of the drive or share shown on the bottom border of each panel.
- **Flat view (Alt+D)**: all files of a folder tree in one list, with every operation available; copy/move put the files directly into the destination and refuse duplicate names.
- The **drive list** moves from Alt+D to **Alt+V**.
- Open-source release: MIT licence, third-party notices, CI workflow, clean module files.

## 1.6 — 2026-10-26

- Recursive total in the size column of marked folders.
- File names coloured by type; yellow reserved for marked items.
- After a move or delete the cursor goes to the next item.
- Selection total on the panel's top border.
- `*` marks everything / unmarks everything.
- Blinking underscore text cursor.
- Status bar with fixed columns and a flexible name column.
- Dark-blue theme.
- Windows Terminal profile (Cascadia Mono) and install script.
- Automatic refresh with file-system change notifications.

## 1.5 — 2026-10-23

- Files copied with the Windows copy engine (`CopyFileExW`): server-side copy where supported, unbuffered I/O for files ≥ 256 MB, long paths.
- Parallel copy engine shared by the whole selection: 4 files at a time, one big file (≥ 64 MB) at a time.
- Moves rename what they can and copy the rest in one batch.

## 1.4 — 2026-10-23

- F12 compares the two panels by name and marks the files missing on the other side.
- The help screen (F1) was removed; the README is the reference.

## 1.3 — 2026-10-23

- Ctrl+→ / Ctrl+← show the current location in the opposite panel.
- F5/F6 copy and move straight to the other panel's folder (no destination prompt).
- Cumulative selection total, with folders measured in the background.

## 1.2 — 2026-10-22

- UNC paths and share lists; Ctrl+letter drive switching; commands on Alt+letter.
- Type-to-jump quick search.
- Progress bar with speed and time left.
- Compact dates and an adaptive name column.
- Browsing inside ZIP/7z archives and extracting the selection.

## 1.1 — 2026-10-22

- ←/→ navigation; English interface; 7z preview and extraction.

## 1.0 — 2026-10-22

- First version: dual panels, copy/move/rename/new folder, Recycle Bin, ZIP, search, filter, preview.
