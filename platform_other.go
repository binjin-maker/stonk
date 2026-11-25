//go:build !windows

package main

import (
	"log"
	"os/exec"
)

func toast(title, body, link string) {
	log.Printf("NOTIFY: %s | %s", title, body)
	exec.Command("notify-send", title, body).Run()
}

func openBrowser(u string) { exec.Command("xdg-open", u).Start() }

func startupEnabled() bool     { return false }
func setStartup(on bool) error { return nil }
