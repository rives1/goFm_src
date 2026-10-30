package main

// Colour schemes: three palettes of RGB colours, switched with Ctrl+F9 (v1.12) and
// remembered in state.json. "navy" (dark blue) is the main one. In every scheme the
// mark colour is reserved: no file type uses it or comes close to it, and every colour
// is readable on the backgrounds it is drawn on (checked by the tests). All colours are
// RGB, so they look the same in every terminal profile; classic 16-colour consoles get
// the nearest palette colour.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func rgb(hex int32) tcell.Color { return tcell.NewHexColor(hex) }

// tagColor is a colour as a tview colour tag value, e.g. "#0A1946".
func tagColor(c tcell.Color) string { return fmt.Sprintf("#%06X", c.Hex()) }

type scheme struct {
	name                      string // saved in state.json
	background                tcell.Color
	panelBorder, activeBorder tcell.Color
	title                     tcell.Color // panel titles (paths)
	text, dim, folder         tcell.Color
	marked                    tcell.Color // reserved for marked items
	cursorBg, cursorFg        tcell.Color // cursor row, active panel
	cursorOffBg, cursorOffFg  tcell.Color // cursor row, inactive panel
	contrastBg                tcell.Color // input fields, buttons, dialogs
	barBg                     tcell.Color // status and key bars
	keyFg, keyBg              tcell.Color // key names in the key bar
	noticeFg, noticeBg        tcell.Color // one-shot notices in the status bar
	label                     tcell.Color // form labels, key names in the summary
	err                       tcell.Color
	families                  [7]tcell.Color // in the order of fileTypes
}

var schemes = []scheme{
	{
		name: "navy", background: rgb(0x0A1946),
		panelBorder: rgb(0x5A6E9A), activeBorder: rgb(0xFFE14D), title: rgb(0xFFFFFF),
		text: rgb(0xD5DEEE), dim: rgb(0x7F8FB4), folder: rgb(0xFFFFFF), marked: rgb(0xFFE14D),
		cursorBg: rgb(0x2AB3C8), cursorFg: rgb(0x06122E), cursorOffBg: rgb(0x2B3F6E), cursorOffFg: rgb(0xFFFFFF),
		contrastBg: rgb(0x1E3A78), barBg: rgb(0x06122E), keyFg: rgb(0x06122E), keyBg: rgb(0x2AB3C8),
		noticeFg: rgb(0x06122E), noticeBg: rgb(0x56D4DD), label: rgb(0x8FD3FF), err: rgb(0xFF7B72),
		families: [7]tcell.Color{rgb(0xFF79C6), rgb(0x5AF78E), rgb(0xC792EA), rgb(0x79C0FF), rgb(0xFF6B6B), rgb(0x4EE6C8), rgb(0x9AA3B5)},
	},
	{
		name: "dark", background: rgb(0x1E1E1E),
		panelBorder: rgb(0x707070), activeBorder: rgb(0xFFE14D), title: rgb(0xFFFFFF),
		text: rgb(0xD4D4D4), dim: rgb(0x8C8C8C), folder: rgb(0xFFFFFF), marked: rgb(0xFFE14D),
		cursorBg: rgb(0x5FA8E8), cursorFg: rgb(0x111111), cursorOffBg: rgb(0x3C3C3C), cursorOffFg: rgb(0xFFFFFF),
		contrastBg: rgb(0x333333), barBg: rgb(0x121212), keyFg: rgb(0x111111), keyBg: rgb(0x5FA8E8),
		noticeFg: rgb(0x111111), noticeBg: rgb(0x7FD1C7), label: rgb(0x9CDCFE), err: rgb(0xF48771),
		families: [7]tcell.Color{rgb(0xFF79C6), rgb(0x5AF78E), rgb(0xC792EA), rgb(0x79C0FF), rgb(0xFF6B6B), rgb(0x4EE6C8), rgb(0x9AA3B5)},
	},
	{
		name: "light", background: rgb(0xF6F8FA),
		panelBorder: rgb(0x808994), activeBorder: rgb(0x0969DA), title: rgb(0x1F2328),
		text: rgb(0x1F2328), dim: rgb(0x636C76), folder: rgb(0x000000), marked: rgb(0x986800),
		cursorBg: rgb(0x0969DA), cursorFg: rgb(0xFFFFFF), cursorOffBg: rgb(0xD8DEE4), cursorOffFg: rgb(0x1F2328),
		contrastBg: rgb(0xE6EAEF), barBg: rgb(0xE6EAEF), keyFg: rgb(0xFFFFFF), keyBg: rgb(0x0969DA),
		noticeFg: rgb(0xFFFFFF), noticeBg: rgb(0x0969DA), label: rgb(0x0550AE), err: rgb(0xB42318),
		families: [7]tcell.Color{rgb(0xBF3989), rgb(0x1A7F37), rgb(0x8250DF), rgb(0x0550AE), rgb(0xD80838), rgb(0x0E7C86), rgb(0x57606A)},
	},
}

// The colours in use, set by applyScheme.
var (
	curScheme                      int
	colBackground                  tcell.Color
	colPanelBorder                 tcell.Color
	colActiveBorder                tcell.Color
	colTitle                       tcell.Color
	colText, colDim, colFolder     tcell.Color
	colMarked                      tcell.Color
	colCursorBg, colCursorFg       tcell.Color
	colCursorOffBg, colCursorOffFg tcell.Color
	colContrastBg, colBarBg        tcell.Color
	colKeyFg, colKeyBg             tcell.Color
	colNoticeFg, colNoticeBg       tcell.Color
	colLabel, colError             tcell.Color
)

func init() { applyScheme(0) }

// schemeIndex returns the index of a scheme by name (0, the main scheme, if unknown).
func schemeIndex(name string) int {
	for i, s := range schemes {
		if s.name == name {
			return i
		}
	}
	return 0
}

// applyScheme sets the colours in use and tview's defaults. tview reads its defaults
// when a primitive is created: primitives that already exist are recoloured by
// FM.recolor.
func applyScheme(i int) {
	s := schemes[i]
	curScheme = i
	colBackground, colPanelBorder, colActiveBorder, colTitle = s.background, s.panelBorder, s.activeBorder, s.title
	colText, colDim, colFolder, colMarked = s.text, s.dim, s.folder, s.marked
	colCursorBg, colCursorFg, colCursorOffBg, colCursorOffFg = s.cursorBg, s.cursorFg, s.cursorOffBg, s.cursorOffFg
	colContrastBg, colBarBg, colKeyFg, colKeyBg = s.contrastBg, s.barBg, s.keyFg, s.keyBg
	colNoticeFg, colNoticeBg, colLabel, colError = s.noticeFg, s.noticeBg, s.label, s.err
	for j := range fileTypes {
		fileTypes[j].color = s.families[j]
	}
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    colBackground,
		ContrastBackgroundColor:     colContrastBg,
		MoreContrastBackgroundColor: colCursorOffBg,
		BorderColor:                 colPanelBorder,
		TitleColor:                  colTitle,
		GraphicsColor:               colPanelBorder,
		PrimaryTextColor:            colText,
		SecondaryTextColor:          colLabel, // form labels
		TertiaryTextColor:           colNoticeBg,
		InverseTextColor:            colBackground,
		ContrastSecondaryTextColor:  colActiveBorder,
	}
}

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
		return colFolder, tcell.AttrBold
	}
	if i := family(e.name); i >= 0 {
		return fileTypes[i].color, tcell.AttrNone
	}
	return colText, tcell.AttrNone
}
