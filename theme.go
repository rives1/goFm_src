package main

// Colour theme: dark-blue background, one colour per family of file types, and a
// mark colour (yellow) that no file type uses. All colours are RGB, so they look
// the same in every terminal profile; classic 16-colour consoles get the nearest
// palette colour.

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func rgb(hex int32) tcell.Color { return tcell.NewHexColor(hex) }

var (
	colBackground   = rgb(0x0A1946) // dark navy: the whole interface
	colPanelBorder  = rgb(0x5A6E9A) // inactive panel border
	colActiveBorder = rgb(0xFFE14D) // active panel border
	colText         = rgb(0xD5DEEE) // default text and plain files
	colDim          = rgb(0x7F8FB4) // hidden and system files, secondary text
	colMarked       = rgb(0xFFE14D) // marked items: reserved, used by no file type
	colCursorBg     = rgb(0x2AB3C8) // cursor row, active panel
	colCursorFg     = rgb(0x06122E)
	colCursorOffBg  = rgb(0x2B3F6E) // cursor row, inactive panel
	colContrastBg   = rgb(0x1E3A78) // input fields, buttons, dialogs
	colBarBg        = rgb(0x06122E) // status and key bars
	colKeyBg        = rgb(0x2AB3C8) // key labels in the key bar
	colNoticeBg     = rgb(0x56D4DD)
	colError        = rgb(0xFF7B72)
)

// fileTypes maps a family of extensions to its colour. None of them is yellow,
// so a marked file (yellow) always stands out.
var fileTypes = []struct {
	kind  iconKind
	color tcell.Color
	exts  string
}{
	{kindArchive, rgb(0xFF79C6), ".zip .7z .rar .tar .gz .tgz .bz2 .xz .zst .cab .iso .lz .lzma"},                                                        // archives: pink
	{kindProgram, rgb(0x5AF78E), ".exe .msi .bat .cmd .ps1 .com .vbs .scr .appx .msix .reg"},                                                             // programs and scripts: green
	{kindImage, rgb(0xC792EA), ".jpg .jpeg .png .gif .bmp .tif .tiff .svg .webp .ico .heic .raw .cr2 .nef .psd .ai"},                                     // images: orchid
	{kindDocument, rgb(0x79C0FF), ".pdf .doc .docx .odt .rtf .txt .md .xls .xlsx .xlsm .ods .csv .ppt .pptx .odp .msg .eml .one"},                        // documents: light blue
	{kindMedia, rgb(0xFF6B6B), ".mp3 .wav .flac .aac .ogg .m4a .wma .mp4 .mkv .avi .mov .wmv .webm .m4v .mpg .mpeg"},                                     // audio and video: red
	{kindCode, rgb(0x4EE6C8), ".go .py .js .ts .c .cpp .h .hpp .cs .java .rs .json .xml .yaml .yml .ini .cfg .conf .toml .html .htm .css .sql .sh .log"}, // code, config, logs: cyan
	{kindSystem, rgb(0x9AA3B5), ".dll .sys .bak .tmp .old .dat .db .lnk"},                                                                                // system and support files: grey-blue
}

// extFamily maps a lower-case extension (".zip") to its index in fileTypes.
var extFamily = func() map[string]int {
	m := map[string]int{}
	for i, ft := range fileTypes {
		for _, e := range strings.Fields(ft.exts) {
			m[e] = i
		}
	}
	return m
}()

// family returns the index in fileTypes of a file name's family, or -1.
func family(name string) int {
	if i, ok := extFamily[strings.ToLower(filepath.Ext(name))]; ok {
		return i
	}
	return -1
}

// entryStyle returns the colour and attributes of a row, before the cursor is applied.
func entryStyle(e entry, marked bool) (tcell.Color, tcell.AttrMask) {
	switch {
	case marked:
		return colMarked, tcell.AttrBold
	case e.hidden:
		return colDim, tcell.AttrNone
	case e.isDir:
		return rgb(0xFFFFFF), tcell.AttrBold
	}
	if i := family(e.name); i >= 0 {
		return fileTypes[i].color, tcell.AttrNone
	}
	return colText, tcell.AttrNone
}

// applyTheme sets tview's defaults; it must run before any primitive is created.
func applyTheme() {
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    colBackground,
		ContrastBackgroundColor:     colContrastBg,
		MoreContrastBackgroundColor: colCursorOffBg,
		BorderColor:                 colPanelBorder,
		TitleColor:                  rgb(0xFFFFFF),
		GraphicsColor:               colPanelBorder,
		PrimaryTextColor:            colText,
		SecondaryTextColor:          rgb(0x8FD3FF), // form labels
		TertiaryTextColor:           colNoticeBg,
		InverseTextColor:            colBackground,
		ContrastSecondaryTextColor:  colActiveBorder,
	}
}
