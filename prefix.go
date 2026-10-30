package main

// Ctrl+K (v1.14): add a prefix to the names of the marked items (files and folders),
// or of the item under the cursor. A dialog asks for the text and shows a preview of
// the new names; an empty text cancels. Characters that names cannot contain are
// refused. An item whose new name already exists is skipped, and the skipped items
// are listed at the end; the others are renamed. Afterwards nothing is marked. In the
// flat view each item stays in its own subfolder.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// badNameChars are the characters a name cannot contain: \ and / always, and on
// Windows also : * ? " < > | (the same rule as F2).
func badNameChars() string {
	if onWindows {
		return `\/:*?"<>|`
	}
	return `\/`
}

// badPrefixMsg explains why a prefix is refused.
func badPrefixMsg() string {
	if onWindows {
		return `The prefix cannot contain \ / : * ? " < > |`
	}
	return `The prefix cannot contain \ or /`
}

// maxPreview is the number of names shown in the preview of Ctrl+K.
const maxPreview = 6

// prefixPreview is the preview text: "old → new" for the first names.
func prefixPreview(names []string, prefix string) string {
	if prefix == "" {
		return "Type the text to put before the names.\nAn empty text cancels."
	}
	if strings.ContainsAny(prefix, badNameChars()) {
		return badPrefixMsg() + "."
	}
	var b strings.Builder
	for i, n := range names {
		if i == maxPreview {
			fmt.Fprintf(&b, "… and %d more", len(names)-maxPreview)
			break
		}
		base := filepath.Base(n)
		b.WriteString(base + " → " + prefix + base + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// prefixResult is what addPrefix did: new relative names by old one, and the
// relative names skipped because the new name already exists.
type prefixResult struct {
	renamed map[string]string
	skipped []string
}

// addPrefix renames each item (a name relative to dir, which may contain a subfolder
// in the flat view) to prefix+name in the same folder. Existing names are skipped;
// other errors go to o.
func (o *opCtx) addPrefix(dir string, names []string, prefix string) prefixResult {
	r := prefixResult{renamed: map[string]string{}}
	for _, rel := range names {
		if o.cancelled() {
			break
		}
		newRel := filepath.Join(filepath.Dir(rel), prefix+filepath.Base(rel))
		from, to := filepath.Join(dir, rel), filepath.Join(dir, newRel)
		if _, err := os.Lstat(to); err == nil {
			r.skipped = append(r.skipped, rel)
			continue
		}
		if err := os.Rename(from, to); err != nil {
			o.fail(from, err)
			continue
		}
		r.renamed[rel] = newRel
		o.step(from)
	}
	return r
}

// skippedText is the message listing the items that were not renamed.
func skippedText(r prefixResult, total int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Renamed %d of %d items.\n\nNot renamed, because an item with the new name already exists:\n\n", len(r.renamed), total)
	for i, s := range r.skipped {
		if i == maxWarnNames {
			fmt.Fprintf(&b, "… and %d more\n", len(r.skipped)-maxWarnNames)
			break
		}
		b.WriteString(filepath.Base(s) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// selectedNames are the names (as listed) of the marked entries, or of the entry
// under the cursor: the same items as selection().
func (p *Panel) selectedNames() []string {
	var out []string
	for _, e := range p.entries {
		if p.marked[e.name] {
			out = append(out, e.name)
		}
	}
	if len(out) == 0 {
		if e := p.current(); e != nil && !e.isUp && !e.isDrive && !e.isShare {
			out = append(out, e.name)
		}
	}
	return out
}

// prefixRename is Ctrl+K.
func (fm *FM) prefixRename() {
	if fm.readOnlyHere() {
		return
	}
	p := fm.active
	names := p.selectedNames()
	if len(names) == 0 {
		return
	}
	cursor := ""
	if e := p.current(); e != nil {
		cursor = e.name
	}
	title := "Add a prefix to " + describe(names)
	fm.promptPreview(title, "Prefix: ", func(s string) string { return prefixPreview(names, s) }, func(prefix string) {
		if prefix == "" {
			return // empty: nothing to do
		}
		if strings.ContainsAny(prefix, badNameChars()) {
			fm.showMessage("Error", badPrefixMsg()+".")
			return
		}
		var res prefixResult
		fm.runTask("Adding \""+prefix+"\" to "+describe(names), func(o *opCtx) {
			res = o.addPrefix(p.dir, names, prefix)
		}, func(*opCtx) {
			if p.flat {
				for i := range p.flatEntries {
					if nw, ok := res.renamed[p.flatEntries[i].name]; ok {
						p.flatEntries[i].name = nw
					}
				}
			}
			p.marked = map[string]bool{} // nothing marked afterwards
			if nw, ok := res.renamed[cursor]; ok {
				cursor = nw
			}
			_ = p.load(cursor)
			if len(res.skipped) > 0 {
				fm.showMessage("Add prefix", skippedText(res, len(names)))
			} else {
				fm.notice = fmt.Sprintf("Renamed %s", plural(int64(len(res.renamed)), "item"))
			}
		})
	})
}

// promptPreview is a text request with a live preview below the field: preview is
// called at every change. Characters in badNameChars cannot be typed.
func (fm *FM) promptPreview(title, label string, preview func(string) string, cb func(string)) {
	var name string
	input := tview.NewInputField().SetLabel(tview.Escape(label)).SetFieldWidth(0)
	input.SetAcceptanceFunc(func(text string, last rune) bool { return !strings.ContainsRune(badNameChars(), last) })
	view := tview.NewTextView().SetText(tview.Escape(preview("")))
	view.SetTextColor(colDim)
	input.SetChangedFunc(func(text string) { view.SetText(tview.Escape(preview(text))) })
	form := tview.NewForm()
	closed := false
	accept := func() {
		if closed {
			return
		}
		closed = true
		v := input.GetText()
		fm.closeModal(name)
		cb(v)
	}
	// As in promptEx: the action waits until Enter has been handled by the form.
	input.SetDoneFunc(func(k tcell.Key) {
		if k == tcell.KeyEnter {
			go fm.app.QueueUpdateDraw(accept)
		}
	})
	form.AddFormItem(input).
		AddButton("OK", accept).
		AddButton("Cancel", func() { fm.closeModal(name) }).
		SetCancelFunc(func() { fm.closeModal(name) })
	box := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 5, 0, true).
		AddItem(view, maxPreview+1, 0, false)
	box.SetBorder(true).SetTitle(" " + tview.Escape(title) + " ")
	name = fm.openModal(box, 76, 5+maxPreview+1+2, form)
}
