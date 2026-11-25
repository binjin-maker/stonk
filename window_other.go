//go:build !windows

package main

func runWindow(arg string)   { openBrowser(appURL()) }
func spawnWindow(arg string) { openBrowser(appURL()) }
func registerProtocol()      {}
func startTray(a *App)       {}
func stopTray()              {}
