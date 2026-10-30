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
	kindDrive // local drive
	kindShare
	kindDriveNet // network (mapped) drive
	kindDriveUSB // removable drive: USB stick or disk, memory card
	kindDriveCD  // CD/DVD drive
	numKinds
)

// Nerd Font codepoints (Font Awesome range, unchanged since Nerd Fonts v2).
var nerdIcons = [numKinds]rune{
	kindFile:     '\uF016', // nf-fa-file_o
	kindArchive:  '\uF1C6', // nf-fa-file_archive_o
	kindProgram:  '\uF120', // nf-fa-terminal
	kindImage:    '\uF1C5', // nf-fa-file_image_o
	kindDocument: '\uF0F6', // nf-fa-file_text_o
	kindMedia:    '\uF144', // nf-fa-play_circle
	kindCode:     '\uF1C9', // nf-fa-file_code_o
	kindSystem:   '\uF013', // nf-fa-cog
	kindFolder:   '\uF07B', // nf-fa-folder
	kindUp:       '\uF062', // nf-fa-arrow_up
	kindDrive:    '\uF0A0', // nf-fa-hdd_o
	kindShare:    '\uF233', // nf-fa-server
	kindDriveNet: '\uF0E8', // nf-fa-sitemap
	kindDriveUSB: '\uF287', // nf-fa-usb
	kindDriveCD:  '\uF192', // nf-fa-dot_circle_o
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
	kindDriveNet: '≈',
	kindDriveUSB: '▬',
	kindDriveCD:  '○',
}

// entryKind classifies a row.
func entryKind(e entry) iconKind {
	switch {
	case e.isUp:
		return kindUp
	case e.isDrive:
		switch e.drive {
		case driveNetwork:
			return kindDriveNet
		case driveRemovable:
			return kindDriveUSB
		case driveOptical:
			return kindDriveCD
		}
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
		return colFolder
	}
	if i := family(e.name); i >= 0 {
		return fileTypes[i].color
	}
	return colText
}
