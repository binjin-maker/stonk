package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var version = "dev"

const updRepo = "binjin-maker/stonk"
const assetName = "Seismo.ex_"

var listener net.Listener
var restarting atomic.Bool

type relInfo struct {
	Tag, URL, Notes string
}

func latestRelease() (*relInfo, error) {
	b, err := httpGet("https://api.github.com/repos/" + updRepo + "/releases/latest")
	if err == nil {
		var r struct {
			TagName string `json:"tag_name"`
			Body    string
			Assets  []struct {
				Name string
				URL  string `json:"browser_download_url"`
			}
		}
		if json.Unmarshal(b, &r) == nil && r.TagName != "" {
			for _, a := range r.Assets {
				if strings.EqualFold(a.Name, assetName) {
					return &relInfo{strings.TrimPrefix(r.TagName, "v"), a.URL, r.Body}, nil
				}
			}
		}
	}
	vb, err2 := httpGet("https://raw.githubusercontent.com/" + updRepo + "/main/version.txt")
	if err2 != nil {
		if err != nil {
			return nil, err
		}
		return nil, err2
	}
	return &relInfo{strings.TrimPrefix(strings.TrimSpace(string(vb)), "v"), "https://raw.githubusercontent.com/" + updRepo + "/main/" + assetName, ""}, nil
}

func newerVersion(remote, local string) bool {
	if local == "dev" {
		return false
	}
	p := func(s string) []int {
		var o []int
		for _, x := range strings.Split(s, ".") {
			n, _ := strconv.Atoi(strings.TrimFunc(x, func(r rune) bool { return r < '0' || r > '9' }))
			o = append(o, n)
		}
		for len(o) < 3 {
			o = append(o, 0)
		}
		return o
	}
	r, l := p(remote), p(local)
	for i := 0; i < 3; i++ {
		if r[i] != l[i] {
			return r[i] > l[i]
		}
	}
	return false
}

func (a *App) setUpd(s string) {
	a.mu.Lock()
	a.updStatus = s
	a.mu.Unlock()
}

func (a *App) checkUpdate(install bool) {
	rel, err := latestRelease()
	if err != nil {
		a.setUpd("Couldn't check for updates (" + err.Error() + ")")
		return
	}
	if !newerVersion(rel.Tag, version) {
		a.setUpd("Up to date (v" + version + ")")
		return
	}
	if !install {
		a.setUpd("Update v" + rel.Tag + " available")
		return
	}
	a.setUpd("Downloading v" + rel.Tag + "…")
	exe, err := os.Executable()
	if err != nil {
		a.setUpd("Update failed: " + err.Error())
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	b, err := httpGet(rel.URL)
	if err != nil || len(b) < 1<<20 || !bytes.HasPrefix(b, []byte("MZ")) {
		a.setUpd("Update download failed")
		return
	}
	newp, oldp := exe+".new", strings.TrimSuffix(exe, filepath.Ext(exe))+".old"
	if err := os.WriteFile(newp, b, 0755); err != nil {
		a.setUpd("Update failed: can't write next to the exe (" + err.Error() + ")")
		return
	}
	os.Remove(oldp)
	if err := os.Rename(exe, oldp); err != nil {
		os.Remove(newp)
		a.setUpd("Update failed: " + err.Error())
		return
	}
	if err := os.Rename(newp, exe); err != nil {
		os.Rename(oldp, exe)
		a.setUpd("Update failed: " + err.Error())
		return
	}
	a.saveStore()
	a.saveCache()
	if sb, err := os.ReadFile(filepath.Join(appDir, "store.json")); err == nil && json.Valid(sb) {
		os.WriteFile(filepath.Join(appDir, "store.backup.json"), sb, 0644)
	}
	notes := strings.TrimSpace(strings.SplitN(rel.Notes, "\n", 2)[0])
	os.WriteFile(filepath.Join(appDir, "updated.txt"), []byte(rel.Tag+"\n"+notes), 0644)
	log.Printf("updated to %s, restarting", rel.Tag)
	restarting.Store(true)
	if listener != nil {
		listener.Close()
	}
	cmd := exec.Command(exe, "--background", "--updated")
	cmd.Dir = filepath.Dir(exe)
	if err := cmd.Start(); err != nil {
		os.Rename(exe, newp)
		os.Rename(oldp, exe)
		if l, err2 := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err2 == nil {
			listener = l
			restarting.Store(false)
			go serveOn(l)
		}
		a.setUpd("Restart failed, kept the old version: " + err.Error())
		return
	}
	os.Exit(0)
}

func (a *App) updateLoop() {
	time.Sleep(8 * time.Second)
	for {
		a.mu.RLock()
		auto := !a.store.Settings.NoAutoUpdate
		a.mu.RUnlock()
		a.checkUpdate(auto)
		time.Sleep(10 * time.Minute)
	}
}

func postUpdateCleanup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	oldp := strings.TrimSuffix(exe, filepath.Ext(exe)) + ".old"
	for i := 0; i < 10; i++ {
		if os.Remove(oldp) == nil || os.IsNotExist(func() error { _, e := os.Stat(oldp); return e }()) {
			break
		}
		time.Sleep(time.Second)
	}
	p := filepath.Join(appDir, "updated.txt")
	if b, err := os.ReadFile(p); err == nil {
		os.Remove(p)
		parts := strings.SplitN(string(b), "\n", 2)
		body := "Seismo restarted on the new version and is still watching your stocks."
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			body = strings.TrimSpace(parts[1])
		}
		toast(fmt.Sprintf("Seismo updated to v%s", parts[0]), body, appURL())
	}
}
