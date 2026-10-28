package main

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every build uses the same compiler: the workflows' GO_VERSION equals the toolchain
// pinned in go.mod (scripts/build-release.sh checks the Go in use against go.mod).
func TestPinnedToolchain(t *testing.T) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^toolchain go(\d+\.\d+\.\d+)$`).FindSubmatch(mod)
	if m == nil {
		t.Fatal("go.mod has no toolchain line")
	}
	if !regexp.MustCompile(`(?m)^tool github.com/tc-hib/go-winres$`).Match(mod) {
		t.Error("go-winres is not a tool of the module")
	}
	for _, wf := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		data, err := os.ReadFile(wf)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "GO_VERSION: '"+string(m[1])+"'") {
			t.Errorf("%s does not build with Go %s", wf, m[1])
		}
		if strings.Contains(string(data), "go-version-file") {
			t.Errorf("%s still reads the version from go.mod's go line", wf)
		}
	}
}

// The Windows resources describe the program and never ask for administrator rights.
func TestWindowsResources(t *testing.T) {
	data, err := os.ReadFile("winres/winres.json")
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Icon     map[string]map[string][]string `json:"RT_GROUP_ICON"`
		Manifest map[string]map[string]struct {
			Level    string `json:"execution-level"`
			UIAccess bool   `json:"ui-access"`
			Elevate  bool   `json:"auto-elevate"`
			LongPath bool   `json:"long-path-aware"`
		} `json:"RT_MANIFEST"`
		Version map[string]map[string]struct {
			Info map[string]map[string]string `json:"info"`
		} `json:"RT_VERSION"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	man := w.Manifest["#1"]["0409"]
	if man.Level != "as invoker" || man.UIAccess || man.Elevate || !man.LongPath {
		t.Errorf("manifest: %+v", man)
	}
	info := w.Version["#1"]["0000"].Info["0409"]
	for k, want := range map[string]string{
		"ProductName": "gofm", "CompanyName": "rives", "OriginalFilename": "gofm.exe",
		"InternalName": "gofm", "LegalCopyright": "Copyright (c) 2026 rives",
	} {
		if info[k] != want {
			t.Errorf("%s = %q, want %q", k, info[k], want)
		}
	}
	if !strings.Contains(info["Comments"], "github.com/rives1/goFm_src") || info["FileDescription"] == "" {
		t.Error("description or link missing")
	}
	for _, f := range w.Icon["APP"]["0000"] {
		if _, err := os.Stat("winres/" + f); err != nil {
			t.Errorf("icon %s: %v", f, err)
		}
	}
}
