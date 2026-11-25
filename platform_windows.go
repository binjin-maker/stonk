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

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func windowOpen() bool { return findAppWindow() != 0 }

func applyUpdate(exe, newp, oldp string, reopen bool) error {
	args := "'--background','--updated'"
	if reopen {
		args += ",'--reopen'"
	}
	ps := `$ErrorActionPreference='SilentlyContinue';$exe=` + psQuote(exe) + `;$new=` + psQuote(newp) + `;$old=` + psQuote(oldp) + `;$me=` + fmt.Sprint(os.Getpid()) + `;
for($i=0;$i -lt 20;$i++){ if(-not (Get-Process -Id $me)){break}; Start-Sleep -Milliseconds 300 }
Get-Process | Where-Object { $_.Path -eq $exe } | Stop-Process -Force
$ok=$false
for($i=0;$i -lt 60;$i++){
  try {
    if(Test-Path -LiteralPath $old){ Remove-Item -LiteralPath $old -Force -ErrorAction Stop }
    Move-Item -LiteralPath $exe -Destination $old -Force -ErrorAction Stop
    try { Move-Item -LiteralPath $new -Destination $exe -Force -ErrorAction Stop; $ok=$true; break }
    catch { Move-Item -LiteralPath $old -Destination $exe -Force; throw }
  } catch { Start-Sleep -Milliseconds 500; Get-Process | Where-Object { $_.Path -eq $exe } | Stop-Process -Force }
}
if(-not $ok){ try { Copy-Item -LiteralPath $new -Destination $exe -Force -ErrorAction Stop; $ok=$true } catch {} }
Start-Process -FilePath $exe -ArgumentList ` + args + ` -WorkingDirectory (Split-Path -LiteralPath $exe)`
	u := utf16.Encode([]rune(ps))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	cmd := hidden(exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-EncodedCommand", base64.StdEncoding.EncodeToString(b)))
	cmd.SysProcAttr.CreationFlags |= 0x00000008 | 0x00000200
	return cmd.Start()
}
