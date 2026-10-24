// Package upgrade lets `dbc upgrade` replace the running binary in place,
// so updating doesn't require re-running install.sh/install.ps1.
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// defaultPagesURL is where dbc's release archives are published (see
// .gitlab-ci.yml's pages job and install.sh/install.ps1). Override with the
// DBC_PAGES_URL environment variable (same variable install.sh honors).
const defaultPagesURL = "http://example.invalid:1688"

func pagesURL() string {
	if url := os.Getenv("DBC_PAGES_URL"); url != "" {
		return url
	}
	return defaultPagesURL
}

// LatestVersion fetches the version tag most recently published to
// releases/latest/VERSION (a plain-text file the pages job writes on every
// tagged release) — cheap enough to call on every command invocation,
// unlike downloading a whole release archive just to check its version.
func LatestVersion(ctx context.Context) (string, error) {
	url := pagesURL() + "/releases/latest/VERSION"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// currentVersion (main.version, injected via goreleaser's {{ .Version }})
	// never has a leading "v" — trim defensively in case the published file
	// ever does, so the two are never falsely reported as different.
	return strings.TrimPrefix(strings.TrimSpace(string(body)), "v"), nil
}

// CheckAndNotify checks (bounded to a few seconds, so it never meaningfully
// delays a command) whether a newer version is published, printing a
// one-line reminder if so. Any failure — network down, Pages unreachable,
// version file missing — is swallowed: this is a courtesy, never something
// that should fail or slow down the caller's real work.
func CheckAndNotify(ctx context.Context, currentVersion string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	latest, err := LatestVersion(ctx)
	if err != nil || latest == "" || latest == currentVersion {
		return
	}
	fmt.Printf("ℹ️  有新版本可用：%s → %s，執行 `dbc upgrade` 更新。\n", currentVersion, latest)
}

// Run downloads the latest release for the current OS/arch and replaces the
// currently running executable (and instances.yaml alongside it) with it.
func Run(ctx context.Context, currentVersion string) error {
	platform, ext, err := platformExt()
	if err != nil {
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("找不到目前執行檔位置: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	installDir := filepath.Dir(exePath)

	// 上次升級在 Windows 上留下的舊執行檔（見 replaceSelf）：那次它還在
	// 跑、檔案還鎖著刪不掉，這次一定已經結束了，順手清掉，非必要不報錯。
	os.Remove(exePath + ".old")

	url := fmt.Sprintf("%s/releases/latest/dbc_%s.%s", pagesURL(), platform, ext)
	fmt.Printf("從 %s 下載最新版本...\n", url)

	tmpDir, err := os.MkdirTemp("", "dbc-upgrade-")
	if err != nil {
		return fmt.Errorf("建立暫存目錄失敗: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "download."+ext)
	if err := downloadToFile(ctx, url, archivePath); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return fmt.Errorf("建立暫存目錄失敗: %w", err)
	}
	if ext == "zip" {
		err = extractZip(archivePath, extractDir)
	} else {
		err = extractTarGz(archivePath, extractDir)
	}
	if err != nil {
		return err
	}

	binName := "dbc"
	if runtime.GOOS == "windows" {
		binName = "dbc.exe"
	}
	newBin := filepath.Join(extractDir, binName)
	if _, err := os.Stat(newBin); err != nil {
		return fmt.Errorf("解壓後找不到 %s", binName)
	}

	if err := replaceSelf(exePath, newBin); err != nil {
		return err
	}

	newInstances := filepath.Join(extractDir, "instances.yaml")
	if _, err := os.Stat(newInstances); err == nil {
		if err := copyFile(newInstances, filepath.Join(installDir, "instances.yaml")); err != nil {
			return fmt.Errorf("更新 instances.yaml 失敗: %w", err)
		}
	}

	newVersion, verErr := detectVersion(ctx, exePath)
	if verErr != nil {
		// 執行檔已經真的換過去了，只是問不出新版本號，不當成失敗。
		fmt.Printf("已更新：%s（原版本 %s，新版本號查詢失敗：%v）\n", exePath, currentVersion, verErr)
		return nil
	}
	fmt.Printf("已更新：%s\n  %s → %s\n", exePath, currentVersion, newVersion)
	return nil
}

// detectVersion runs the (already-replaced) binary's --version and strips
// cobra's "dbc version " prefix, so it prints symmetrically with
// currentVersion (which is just the bare version string).
func detectVersion(ctx context.Context, exePath string) (string, error) {
	out, err := exec.CommandContext(ctx, exePath, "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "dbc version "), nil
}

func platformExt() (platform, ext string, err error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		ext = "tar.gz"
	case "windows":
		ext = "zip"
	default:
		return "", "", fmt.Errorf("不支援的作業系統: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64", "arm64":
	default:
		return "", "", fmt.Errorf("不支援的架構: %s", runtime.GOARCH)
	}

	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		return "", "", fmt.Errorf("windows/arm64 沒有官方 build")
	}

	return runtime.GOOS + "_" + runtime.GOARCH, ext, nil
}

func downloadToFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("下載失敗: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下載失敗: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("建立暫存檔失敗: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("寫入暫存檔失敗: %w", err)
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("解壓縮失敗: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("解壓縮失敗: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		// 上游 archive 是扁平結構（沒有子目錄），直接用檔名。
		target := filepath.Join(destDir, filepath.Base(hdr.Name))
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return fmt.Errorf("解壓縮失敗: %w", err)
		}
		_, err = io.Copy(out, tr)
		closeErr := out.Close()
		if err != nil {
			return fmt.Errorf("解壓縮失敗: %w", err)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("解壓縮失敗: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("解壓縮失敗: %w", err)
		}
		target := filepath.Join(destDir, filepath.Base(f.Name))
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("解壓縮失敗: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("解壓縮失敗: %w", copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// replaceSelf atomically swaps target (the currently running executable)
// for newBin's content.
//
// On Unix, renaming straight over target works: rename only repoints the
// directory entry, and the running process keeps its already-open inode
// regardless. Windows doesn't allow that — a running executable's mapped
// image can be *renamed* to a different name (that's a plain rename, no
// content change), but renaming a *different* file onto its existing path
// (i.e. replacing it) is rejected outright, with ERROR_ACCESS_DENIED
// ("Access is denied") — confirmed against a real Windows user hitting
// exactly that error. So the swap always goes in two rename-only steps,
// which works on both platforms: move target out of the way first
// (target -> target+".old", freeing up its name), then move the new binary
// into that now-empty name. The displaced ".old" file is still the exact
// file this process is running from, so it can't be deleted yet — Run
// cleans up a leftover one from the *previous* upgrade at the start of the
// next one, once that old process has long since exited.
func replaceSelf(target, newBin string) error {
	staged := target + ".new"
	if err := copyFile(newBin, staged); err != nil {
		return fmt.Errorf("準備新版本失敗: %w", err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		os.Remove(staged)
		return fmt.Errorf("設定執行權限失敗: %w", err)
	}

	old := target + ".old"
	os.Remove(old) // 正常不該存在，保險起見先清掉，避免撞到下面的 rename
	if err := os.Rename(target, old); err != nil {
		os.Remove(staged)
		return fmt.Errorf("搬移舊執行檔失敗: %w", err)
	}
	if err := os.Rename(staged, target); err != nil {
		// 盡量復原，不要讓使用者卡在「舊的不見了、新的也沒進來」的狀態。
		os.Rename(old, target)
		os.Remove(staged)
		return fmt.Errorf("替換執行檔失敗: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
