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
	background, updated := false, false
	for _, a := range os.Args[1:] {
		if a == "--background" {
			background = true
		}
		if a == "--updated" {
			updated = true
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	appDir = filepath.Join(base, "Seismo")
	os.MkdirAll(appDir, 0755)
	if lf, err := os.OpenFile(filepath.Join(appDir, "seismo.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644); err == nil {
		log.SetOutput(lf)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	for i := 0; err != nil && updated && i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		if !background {
			openBrowser(appURL())
		}
		return
	}
	listener = ln
	app := NewApp()
	app.Load()
	go app.Run()
	go app.updateLoop()
	go postUpdateCleanup()
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	app.Routes(mux)
	if !background && os.Getenv("SEISMO_NOBROWSER") == "" {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(appURL())
		}()
	}
	if background && !updated {
		go toast("Seismo is watching the markets", "Running in the background. You'll be notified about big events on your stocks.", appURL())
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
