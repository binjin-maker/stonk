//go:build !windows

package main

import (
	"log"
	"os"
	"os/exec"
)

func toast(title, body, link string) {
	log.Printf("NOTIFY: %s | %s", title, body)
	exec.Command("notify-send", title, body).Run()
}

func openBrowser(u string) { exec.Command("xdg-open", u).Start() }

func startupEnabled() bool     { return false }
func setStartup(on bool) error { return nil }

func windowOpen() bool { return false }

func applyUpdate(exe, newp, oldp string, reopen bool) error {
	os.Remove(oldp)
	if err := os.Rename(exe, oldp); err != nil {
		return err
	}
	if err := os.Rename(newp, exe); err != nil {
		os.Rename(oldp, exe)
		return err
	}
	return exec.Command(exe, "--background", "--updated").Start()
}
