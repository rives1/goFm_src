package main

// Windows path rules (UNC shares, drive roots) written with plain string
// operations, so they can be unit-tested on any OS.

import (
	"runtime"
	"strings"
)

var onWindows = runtime.GOOS == "windows"

// uncParts splits \\server\share\rest. ok is false for non-UNC paths.
// Forward slashes are accepted too (//server/share).
func uncParts(p string) (server, share, rest string, ok bool) {
	p = strings.ReplaceAll(p, "/", `\`)
	if !strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\\.\`) {
		return "", "", "", false
	}
	parts := strings.SplitN(strings.TrimLeft(p, `\`), `\`, 3)
	if parts[0] == "" {
		return "", "", "", false
	}
	server = parts[0]
	if len(parts) > 1 {
		share = parts[1]
	}
	if len(parts) > 2 {
		rest = strings.Trim(parts[2], `\`)
	}
	return server, share, rest, true
}

// isUNCServer reports whether p names a server only (\\server), i.e. the share list.
func isUNCServer(p string) bool {
	_, share, _, ok := uncParts(p)
	return ok && share == ""
}

// isUNCShareRoot reports whether p is the root of a share (\\server\share or \\server\share\).
func isUNCShareRoot(p string) bool {
	_, share, rest, ok := uncParts(p)
	return ok && share != "" && rest == ""
}

// normalizeUNC returns \\server, or \\server\share\ for share roots, or the path itself.
func normalizeUNC(p string) string {
	server, share, rest, ok := uncParts(p)
	switch {
	case !ok:
		return p
	case share == "":
		return `\\` + server
	case rest == "":
		return `\\` + server + `\` + share + `\`
	default:
		return `\\` + server + `\` + share + `\` + rest
	}
}

// windowsParent returns the parent of a Windows location and the name to select in it.
// Locations: "" = drive list, \\server = share list, otherwise a folder.
func windowsParent(dir string) (parent, selectName string, ok bool) {
	if dir == "" {
		return "", "", false
	}
	if server, share, rest, isUNC := uncParts(dir); isUNC {
		switch {
		case share == "":
			return "", "", true // share list -> drive list
		case rest == "":
			return `\\` + server, share, true // share root -> share list
		default:
			i := strings.LastIndex(rest, `\`)
			if i < 0 {
				return `\\` + server + `\` + share + `\`, rest, true
			}
			return `\\` + server + `\` + share + `\` + rest[:i], rest[i+1:], true
		}
	}
	d := strings.TrimRight(strings.ReplaceAll(dir, "/", `\`), `\`)
	if len(d) == 2 && d[1] == ':' { // drive root
		return "", strings.ToUpper(d[:1]) + `:\`, true
	}
	i := strings.LastIndex(d, `\`)
	if i < 0 {
		return "", "", false
	}
	parent = d[:i]
	if len(parent) == 2 && parent[1] == ':' {
		parent += `\`
	}
	return parent, d[i+1:], true
}

// driveLetter returns the upper-case drive letter of a path like D:\x, or 0.
func driveLetter(p string) byte {
	if len(p) >= 2 && p[1] == ':' {
		c := p[0]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c >= 'A' && c <= 'Z' {
			return c
		}
	}
	return 0
}

type shareEntry struct {
	name    string
	remark  string
	special bool // administrative share such as C$ or ADMIN$
}
