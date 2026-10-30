package main

// Kinds of drive (v1.12): shown in the drive list with their own icon and label, and
// used by Del to avoid the Recycle Bin on network drives.

type driveType int

const (
	driveLocal     driveType = iota // internal disk (also RAM disks and unknown types)
	driveNetwork                    // mapped network drive
	driveRemovable                  // USB stick or disk, memory card, floppy
	driveOptical                    // CD/DVD/Blu-ray
)

// label is the text of the size column in the drive list (at most 9 cells).
func (d driveType) label() string {
	switch d {
	case driveNetwork:
		return "NETWORK"
	case driveRemovable:
		return "REMOVABLE"
	case driveOptical:
		return "CD/DVD"
	}
	return "LOCAL"
}

// isNetworkLocation reports whether a folder is on a network: a UNC path
// (\\server\share\...) or a folder on a mapped network drive.
func isNetworkLocation(dir string) bool {
	if !onWindows || dir == "" {
		return false
	}
	if _, _, _, unc := uncParts(dir); unc {
		return true
	}
	if l := driveLetter(dir); l != 0 {
		return driveKind(string(l)+`:\`) == driveNetwork
	}
	return false
}
