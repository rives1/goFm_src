package main

// File-type icons: one per family of fileTypes, plus folder, plain file, "..",
// drive and share. Two sets exist:
//
//   - Nerd Font glyphs (default): real icons, but only with a font that contains
//     them, e.g. "Cascadia Mono NF". With another font they show as empty boxes,
//     and a terminal program cannot detect that, so the user switches with Alt+I.
//   - basic symbols: characters of the classic PC character set (code page 437),
//     present in Cascadia Mono, Consolas and Lucida Console, so they work in the
//     classic console too.
//
// Every icon is exactly one cell wide. The icon has the colour of its family even
// when the item is marked; on the cursor row of the active panel it takes the
// cursor text colour, because some family colours are unreadable on the cursor
// background.

import "github.com/gdamore/tcell/v2"

type iconKind int

const (
	kindFile iconKind = iota // a file of no known family
	kindArchive
	kindProgram
	kindImage
	kindDocument
	kindMedia
	kindCode
	kindSystem
	kindFolder
	kindUp
	kindDrive
	kindShare
	numKinds
)

// Nerd Font codepoints (Font Awesome range, unchanged since Nerd Fonts v2).
var nerdIcons = [numKinds]rune{
	kindFile:     '', // nf-fa-file_o
	kindArchive:  '', // nf-fa-file_archive_o
	kindProgram:  '', // nf-fa-terminal
	kindImage:    '', // nf-fa-file_image_o
	kindDocument: '', // nf-fa-file_text_o
	kindMedia:    '', // nf-fa-play_circle
	kindCode:     '', // nf-fa-file_code_o
	kindSystem:   '', // nf-fa-cog
	kindFolder:   '', // nf-fa-folder
	kindUp:       '', // nf-fa-arrow_up
	kindDrive:    '', // nf-fa-hdd_o
	kindShare:    '', // nf-fa-server
}

// Basic symbols, all in code page 437.
var basicIcons = [numKinds]rune{
	kindFile:     '·',
	kindArchive:  '▒',
	kindProgram:  '♦',
	kindImage:    '☼',
	kindDocument: '¶',
	kindMedia:    '♫',
	kindCode:     '§',
	kindSystem:   '◘',
	kindFolder:   '►',
	kindUp:       '↑',
	kindDrive:    '■',
	kindShare:    '≡',
}

// entryKind classifies a row.
func entryKind(e entry) iconKind {
	switch {
	case e.isUp:
		return kindUp
	case e.isDrive:
		return kindDrive
	case e.isShare:
		return kindShare
	case e.isDir:
		return kindFolder
	}
	if i := family(e.name); i >= 0 {
		return fileTypes[i].kind
	}
	return kindFile
}

func iconFor(e entry, basic bool) rune {
	if basic {
		return basicIcons[entryKind(e)]
	}
	return nerdIcons[entryKind(e)]
}

// iconColor is the family colour, never the mark colour. Hidden items are dimmed
// like their name.
func iconColor(e entry) tcell.Color {
	switch {
	case e.hidden:
		return colDim
	case e.isDir:
		return rgb(0xFFFFFF)
	}
	if i := family(e.name); i >= 0 {
		return fileTypes[i].color
	}
	return colText
}
