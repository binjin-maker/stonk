# Seismo (stonk)

Native Windows app (own window + system tray) that scans worldwide news for events hitting big companies, predicts stock moves, tracks your investments 24/7, and sends Windows notifications for big events.

**Download:** grab `Seismo.ex_` from the latest release (or the repo root), rename to `Seismo.exe`, run it.

The app updates itself from this repo's releases. Your portfolio, watchlist, alerts and corrections live in `%APPDATA%\Seismo` and are never touched by updates (backed up before every update).

Build: `./build.sh 1.2.3` → `Seismo.ex_` with icon and version baked in. Push `Seismo.ex_`, `version.txt` and `notes.txt` and every install updates within ~10 minutes.

Uses the WebView2 runtime built into Windows 10/11 (falls back to the browser if it's missing).
