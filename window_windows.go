//go:build windows

package main

import (
	_ "embed"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"fyne.io/systray"
	webview2 "github.com/jchv/go-webview2"
)

//go:embed web/tray.ico
var trayIcon []byte

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procFindWindow = user32.NewProc("FindWindowW")
	procSetFg      = user32.NewProc("SetForegroundWindow")
	procShowWindow = user32.NewProc("ShowWindow")
	procIsIconic   = user32.NewProc("IsIconic")
	procAllowSetFg = user32.NewProc("AllowSetForegroundWindow")
)

func findAppWindow() uintptr {
	cls, _ := syscall.UTF16PtrFromString("webview")
	h, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(cls)), 0)
	return h
}

func routeFromArg(arg string) string {
	if !strings.HasPrefix(strings.ToLower(arg), "seismo://") {
		return ""
	}
	u, err := url.Parse(arg)
	if err != nil {
		return ""
	}
	p := strings.Trim(u.Host+u.Path, "/")
	parts := strings.Split(p, "/")
	if len(parts) >= 2 && parts[0] == "t" {
		return "t=" + url.QueryEscape(strings.ToUpper(parts[1]))
	}
	if len(parts) >= 1 && parts[0] == "alerts" {
		return "alerts"
	}
	if len(parts) >= 1 && parts[0] == "ipo" {
		return "ipo"
	}
	return ""
}

func runWindow(arg string) {
	route := routeFromArg(arg)
	if h := findAppWindow(); h != 0 {
		if route != "" {
			httpPost(appURL()+"api/nav", `{"Hash":"`+route+`"}`)
		}
		if r, _, _ := procIsIconic.Call(h); r != 0 {
			procShowWindow.Call(h, 9)
		}
		procSetFg.Call(h)
		return
	}
	runtime.LockOSThread()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(appDir, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  "Seismo",
			Width:  1440,
			Height: 920,
			IconId: 1,
			Center: true,
		},
	})
	if w == nil {
		openBrowser(appURL() + hashPart(route))
		return
	}
	defer w.Destroy()
	w.SetSize(1100, 700, webview2.HintMin)
	w.Bind("openExternal", func(u string) {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			openBrowser(u)
		}
	})
	w.Bind("closeWindow", func() { w.Terminate() })
	w.Navigate(appURL() + hashPart(route))
	w.Run()
	firstClose := filepath.Join(appDir, "closed-once")
	if _, err := os.Stat(firstClose); os.IsNotExist(err) {
		os.WriteFile(firstClose, []byte("1"), 0644)
		toast("Seismo is still watching your stocks", "It keeps running in the system tray. Click the tray icon to reopen it, or right-click it to quit.", "seismo://open")
	}
}

func hashPart(route string) string {
	if route == "" {
		return ""
	}
	return "#" + route
}

func spawnWindow(arg string) {
	exe, err := os.Executable()
	if err != nil {
		openBrowser(appURL())
		return
	}
	procAllowSetFg.Call(^uintptr(0))
	args := []string{"--window"}
	if arg != "" {
		args = append(args, arg)
	}
	exec.Command(exe, args...).Start()
}

func registerProtocol() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	k := `HKCU\Software\Classes\seismo`
	hidden(exec.Command("reg", "add", k, "/ve", "/d", "URL:Seismo", "/f")).Run()
	hidden(exec.Command("reg", "add", k, "/v", "URL Protocol", "/d", "", "/f")).Run()
	hidden(exec.Command("reg", "add", k+`\DefaultIcon`, "/ve", "/d", `"`+exe+`",0`, "/f")).Run()
	hidden(exec.Command("reg", "add", k+`\shell\open\command`, "/ve", "/d", `"`+exe+`" --window "%1"`, "/f")).Run()
}

func startTray(a *App) {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTooltip("Seismo: watching your stocks")
			systray.SetOnTapped(func() { spawnWindow("") })
			mOpen := systray.AddMenuItem("Open Seismo", "Open the Seismo window")
			mScan := systray.AddMenuItem("Scan news now", "Run a full news scan now")
			mAlerts := systray.AddMenuItem("Alerts", "Show recent alerts")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Quit Seismo", "Stop tracking and notifications")
			for {
				select {
				case <-mOpen.ClickedCh:
					spawnWindow("")
				case <-mScan.ClickedCh:
					select {
					case a.trigger <- struct{}{}:
					default:
					}
				case <-mAlerts.ClickedCh:
					spawnWindow("seismo://alerts")
				case <-mQuit.ClickedCh:
					a.shutdown()
				}
			}
		}, func() {})
	}()
}

func stopTray() { systray.Quit() }
