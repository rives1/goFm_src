package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCopyTree(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "src", "a.txt"), "A")
	write(t, filepath.Join(d, "src", "sub", "b.txt"), "B")
	o := &opCtx{}
	o.copyPath(filepath.Join(d, "src"), filepath.Join(d, "dst"))
	if len(o.errs) > 0 {
		t.Fatal(o.errs)
	}
	if read(t, filepath.Join(d, "dst", "sub", "b.txt")) != "B" || read(t, filepath.Join(d, "src", "a.txt")) != "A" {
		t.Fatal("copia errata")
	}
}

func TestMoveMergeKeepsOnlyCompleteCopies(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "src", "x", "new.txt"), "N")
	write(t, filepath.Join(d, "src", "x", "same.txt"), "SRC")
	write(t, filepath.Join(d, "dst", "x", "same.txt"), "OLD")
	write(t, filepath.Join(d, "dst", "x", "keep.txt"), "K")
	o := &opCtx{}
	o.movePath(filepath.Join(d, "src", "x"), filepath.Join(d, "dst", "x"))
	if len(o.errs) > 0 {
		t.Fatal(o.errs)
	}
	if exists(filepath.Join(d, "src", "x")) {
		t.Fatal("origine non rimossa dopo fusione riuscita")
	}
	if read(t, filepath.Join(d, "dst", "x", "same.txt")) != "SRC" || read(t, filepath.Join(d, "dst", "x", "keep.txt")) != "K" {
		t.Fatal("fusione errata")
	}
}

func TestMoveRename(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "f.txt"), "F")
	o := &opCtx{}
	o.movePath(filepath.Join(d, "f.txt"), filepath.Join(d, "g.txt"))
	if exists(filepath.Join(d, "f.txt")) || read(t, filepath.Join(d, "g.txt")) != "F" {
		t.Fatal("spostamento errato")
	}
}

func TestZipRoundTrip(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "dir", "città.txt"), "accentato")
	write(t, filepath.Join(d, "dir", "sub", "z.txt"), "Z")
	write(t, filepath.Join(d, "solo.txt"), "S")
	zp := filepath.Join(d, "out.zip")
	o := &opCtx{}
	o.zipPaths([]string{filepath.Join(d, "dir"), filepath.Join(d, "solo.txt")}, d, zp)
	if len(o.errs) > 0 {
		t.Fatal(o.errs)
	}
	o.extract(zp, filepath.Join(d, "x"), false)
	if len(o.errs) > 0 {
		t.Fatal(o.errs)
	}
	if read(t, filepath.Join(d, "x", "dir", "città.txt")) != "accentato" ||
		read(t, filepath.Join(d, "x", "dir", "sub", "z.txt")) != "Z" ||
		read(t, filepath.Join(d, "x", "solo.txt")) != "S" {
		t.Fatal("round trip zip errato")
	}
	// Salta esistenti non deve sovrascrivere
	write(t, filepath.Join(d, "x", "solo.txt"), "MODIFICATO")
	o.extract(zp, filepath.Join(d, "x"), false)
	if read(t, filepath.Join(d, "x", "solo.txt")) != "MODIFICATO" {
		t.Fatal("skip esistenti non rispettato")
	}
	o.extract(zp, filepath.Join(d, "x"), true)
	if read(t, filepath.Join(d, "x", "solo.txt")) != "S" {
		t.Fatal("sovrascrittura non applicata")
	}
}

func TestZipIncludingItself(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a.txt"), "A")
	zp := filepath.Join(d, "self.zip")
	o := &opCtx{}
	o.zipPaths([]string{d}, filepath.Dir(d), zp)
	r, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "self.zip") {
			t.Fatal("l'archivio contiene sé stesso")
		}
	}
}

func TestZipSlipBlocked(t *testing.T) {
	d := t.TempDir()
	zp := filepath.Join(d, "evil.zip")
	f, _ := os.Create(zp)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("../../evil.txt")
	w.Write([]byte("x"))
	w, _ = zw.Create("ok.txt")
	w.Write([]byte("ok"))
	zw.Close()
	f.Close()
	o := &opCtx{}
	o.extract(zp, filepath.Join(d, "out"), true)
	if exists(filepath.Join(d, "evil.txt")) || exists(filepath.Join(filepath.Dir(d), "evil.txt")) {
		t.Fatal("zip slip non bloccato")
	}
	if len(o.errs) != 1 || read(t, filepath.Join(d, "out", "ok.txt")) != "ok" {
		t.Fatal("comportamento inatteso", o.errs)
	}
}

func TestSearch(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "a", "Report.PDF"), "x")
	write(t, filepath.Join(d, "b", "note.txt"), "riga uno\nParola Chiave qui\n")
	write(t, filepath.Join(d, ".nascosta", "x.pdf"), "x")
	o := &opCtx{}
	if h := o.search(d, "*.pdf", "", false, 100); len(h) != 1 {
		t.Fatalf("attesi 1 pdf, trovati %v", h)
	}
	if h := o.search(d, "*.pdf", "", true, 100); len(h) != 2 {
		t.Fatalf("attesi 2 pdf con nascosti, trovati %v", h)
	}
	if h := o.search(d, "", "parola chiave", false, 100); len(h) != 1 || !strings.HasSuffix(h[0].path, "note.txt") {
		t.Fatalf("ricerca contenuto: %v", h)
	}
}

func TestFileContainsAcrossChunks(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "big.txt")
	data := strings.Repeat("a", 256<<10-3) + "NEEDLE" + strings.Repeat("b", 1000)
	write(t, p, data)
	if !fileContains(p, []byte("needle"), &opCtx{}) {
		t.Fatal("ago a cavallo dei blocchi non trovato")
	}
}

func TestPathHelpers(t *testing.T) {
	if !isWithin("/a/b", "/a/b/c") || isWithin("/a/b", "/a/bc") || !isWithin("/a/b", "/a/b") {
		t.Fatal("isWithin")
	}
	if !matchName("Foto.JPG", "*.jpg") || !matchName("relazione finale.docx", "FINALE") || matchName("a.txt", "*.pdf") {
		t.Fatal("matchName")
	}
}

func TestDecodeText(t *testing.T) {
	if s, ok := decodeText([]byte{0xFF, 0xFE, 'c', 0, 'i', 0, 'a', 0, 'o', 0}); !ok || s != "ciao" {
		t.Fatalf("utf16: %q", s)
	}
	if s, ok := decodeText([]byte("perch\xe8")); !ok || s != "perché" && s != "perchè" {
		t.Fatalf("cp1252: %q", s)
	}
	if _, ok := decodeText([]byte{0x4d, 0x5a, 0x00, 0x01}); ok {
		t.Fatal("binario riconosciuto come testo")
	}
}

func TestTrashLinux(t *testing.T) {
	if onWindows {
		t.Skip("would use the real Recycle Bin")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	d := t.TempDir()
	write(t, filepath.Join(d, "del.txt"), "x")
	if err := moveToTrash([]string{filepath.Join(d, "del.txt")}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(d, "del.txt")) || !exists(filepath.Join(home, ".local/share/Trash/files/del.txt")) {
		t.Fatal("cestino")
	}
}

func TestZipSingleRoot(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "p", "a.txt"), "A")
	write(t, filepath.Join(d, "p", "s", "b.txt"), "B")
	write(t, filepath.Join(d, "q.txt"), "Q")
	o := &opCtx{}
	z1, z2 := filepath.Join(d, "1.zip"), filepath.Join(d, "2.zip")
	o.zipPaths([]string{filepath.Join(d, "p")}, d, z1)
	o.zipPaths([]string{filepath.Join(d, "p"), filepath.Join(d, "q.txt")}, d, z2)
	if !archiveSingleRoot(z1) || archiveSingleRoot(z2) {
		t.Fatal("zipSingleRoot")
	}
}

func Test7zExtractAndPreview(t *testing.T) {
	d := t.TempDir()
	o := &opCtx{}
	o.extract("testdata/single.7z", d, false)
	if len(o.errs) > 0 {
		t.Fatal(o.errs)
	}
	if read(t, filepath.Join(d, "progetto", "città.txt")) != "accentato" ||
		len(read(t, filepath.Join(d, "progetto", "sub", "big.txt"))) != 5*50000 {
		t.Fatal("7z content mismatch")
	}
	if !archiveSingleRoot("testdata/single.7z") || archiveSingleRoot("testdata/multi.7z") {
		t.Fatal("archiveSingleRoot 7z")
	}
	if p := previewArchive("testdata/multi.7z"); !strings.Contains(p, "7z archive") || !strings.Contains(p, "solo.txt") {
		t.Fatal(p)
	}
}

func Test7zEncryptedReportsPassword(t *testing.T) {
	o := &opCtx{}
	o.extract("testdata/secret.7z", t.TempDir(), false)
	if len(o.errs) == 0 || !strings.Contains(strings.Join(o.errs, " "), "password") {
		t.Fatalf("expected password error, got %v", o.errs)
	}
	t.Log(o.errs)
}
