//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func toast(title, body, link string) {
	x := fmt.Sprintf(`<toast activationType="protocol" launch="%s" duration="long"><visual><binding template="ToastGeneric"><text>%s</text><text>%s</text></binding></visual><audio src="ms-winsoundevent:Notification.Default"/><actions><action content="Open Seismo" activationType="protocol" arguments="%s"/></actions></toast>`,
		xmlEsc(link), xmlEsc(title), xmlEsc(body), xmlEsc(link))
	ps := `[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null;` +
		`[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null;` +
		`$x = New-Object Windows.Data.Xml.Dom.XmlDocument;$x.LoadXml('` + strings.ReplaceAll(x, "'", "''") + `');` +
		`$t = New-Object Windows.UI.Notifications.ToastNotification $x;` +
		`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe').Show($t)`
	u := utf16.Encode([]rune(ps))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	hidden(exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))).Run()
}

func openBrowser(u string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

func startupEnabled() bool {
	return hidden(exec.Command("reg", "query", runKey, "/v", "Seismo")).Run() == nil
}

func setStartup(on bool) error {
	if !on {
		hidden(exec.Command("reg", "delete", runKey, "/v", "Seismo", "/f")).Run()
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return hidden(exec.Command("reg", "add", runKey, "/v", "Seismo", "/t", "REG_SZ", "/d", `"`+exe+`" --background`, "/f")).Run()
}
