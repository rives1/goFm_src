# Changelog

All notable changes to gofm. Versions follow the order in which features were added.

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
