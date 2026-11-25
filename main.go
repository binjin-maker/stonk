package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
	_ "time/tzdata"
)

//go:embed web
var webFS embed.FS

const port = 47823

var appDir string

func main() {
	background, updated, window, winArg, reopen := false, false, false, "", false
	for i, a := range os.Args[1:] {
		if a == "--window" {
			window = true
			if i+2 < len(os.Args) {
				winArg = os.Args[i+2]
			}
		}
		if a == "--background" {
			background = true
		}
		if a == "--updated" {
			updated = true
		}
		if a == "--reopen" {
			reopen = true
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	appDir = filepath.Join(base, "Seismo")
	os.MkdirAll(appDir, 0755)
	logName := "seismo.log"
	if window {
		logName = "window.log"
	}
	if lf, err := os.OpenFile(filepath.Join(appDir, logName), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		log.SetOutput(lf)
	}
	if window {
		runWindow(winArg)
		return
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	for i := 0; err != nil && updated && i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		if !background {
			runWindow("")
		}
		return
	}
	listener = ln
	app := NewApp()
	app.Load()
	go app.Run()
	go app.updateLoop()
	go postUpdateCleanup(reopen)
	go registerProtocol()
	startTray(app)
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	app.Routes(mux)
	if !background && os.Getenv("SEISMO_NOBROWSER") == "" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			spawnWindow("")
		}()
	}
	if background && !updated {
		go toast("Seismo is watching the markets", "Running in the tray. You'll be notified about big events on your stocks.", "seismo://open")
	}
	log.Println("listening on", appURL())
	mainMux = mux
	serveOn(ln)
	for restarting.Load() {
		time.Sleep(time.Second)
	}
	select {}
}

var mainMux *http.ServeMux

func serveOn(l net.Listener) {
	err := http.Serve(l, mainMux)
	if !restarting.Load() {
		log.Println("server stopped:", err)
	}
}
