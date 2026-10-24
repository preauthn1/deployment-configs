#!/usr/bin/env bash
# 下載並安裝 dbc（支援 macOS / Linux；Windows 請照 README 手動下載或用
# install.ps1）。
#
# 用法：
#   curl -fsSL http://example.invalid:1688/install.sh | bash
#   curl -fsSL .../install.sh | bash -s -- v0.0.2   # 安裝指定版本
#
# 透過 GitLab Pages 公開發布（repo 本身仍是 private，只有 Pages 內容
# 公開），不需要 GITLAB_TOKEN。
set -euo pipefail

PAGES_URL="${DBC_PAGES_URL:-http://example.invalid:1688}"
VERSION="${1:-latest}"
INSTALL_DIR="${DBC_INSTALL_DIR:-$HOME/.dbc}"
BIN_DIR="${DBC_BIN_DIR:-$HOME/.local/bin}"

log() { echo "$@" >&2; }
die() { log "錯誤：$*"; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || die "找不到 $1，請先安裝"
}

detect_platform() {
  local os arch os_name arch_name
  os="$(uname -s)"
  arch="$(uname -m)"

  case "$os" in
    Darwin) os_name="darwin" ;;
    Linux) os_name="linux" ;;
    *) die "install.sh 不支援 $os，Windows 請照 README 手動下載安裝。" ;;
  esac

  case "$arch" in
    x86_64) arch_name="amd64" ;;
    arm64|aarch64) arch_name="arm64" ;;
    *) die "不支援的架構: $arch" ;;
  esac

  echo "${os_name}_${arch_name}"
}

resolve_url() {
  local platform="$1"
  if [ "$VERSION" = "latest" ]; then
    echo "$PAGES_URL/releases/latest/dbc_${platform}.tar.gz"
  else
    echo "$PAGES_URL/releases/$VERSION/dbc_${VERSION#v}_${platform}.tar.gz"
  fi
}

install_from_archive() {
  local archive="$1"
  # 注意：不要用 `trap ... RETURN` 清理 tmp——這個 trap 是全域註冊的，
  # 不會隨這個函式返回自動失效，呼叫端（main）自己返回時會再次觸發，
  # 這時候 tmp 這個區域變數已經不在作用域內，set -u 底下就會直接噴
  # "tmp: unbound variable"。改成函式結尾手動清理。
  local tmp
  tmp="$(mktemp -d)"

  tar -xzf "$archive" -C "$tmp"
  [ -f "$tmp/dbc" ] || die "解壓後找不到 dbc 執行檔"

  mkdir -p "$INSTALL_DIR" "$BIN_DIR"
  mv "$tmp/dbc" "$INSTALL_DIR/dbc"
  chmod +x "$INSTALL_DIR/dbc"
  if [ -f "$tmp/instances.yaml" ]; then
    mv "$tmp/instances.yaml" "$INSTALL_DIR/instances.yaml"
  fi
  ln -sf "$INSTALL_DIR/dbc" "$BIN_DIR/dbc"
  rm -rf "$tmp"

  log "已安裝：$INSTALL_DIR/dbc（捷徑：$BIN_DIR/dbc）"
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *)
      log "⚠️  $BIN_DIR 不在 PATH，請加進 shell 設定檔（~/.zshrc 或 ~/.bashrc）："
      log "    export PATH=\"$BIN_DIR:\$PATH\""
      ;;
  esac
}

main() {
  require curl
  require tar

  local platform url tmp_archive
  platform="$(detect_platform)" || exit 1
  url="$(resolve_url "$platform")"

  log "安裝 dbc $VERSION ($platform) ..."
  log "從 $url 下載..."

  # 注意：同上，不用 trap ... EXIT 清理——這個 trap 要等整個 shell
  # 真的結束才觸發，那時候 main 可能早就返回了，local 變數已經釋放，
  # 一樣會炸 unbound variable。改成正常執行完手動清理。
  tmp_archive="$(mktemp)"
  curl -fsSL -o "$tmp_archive" "$url" || die "下載失敗，確認版本號或 Pages 是否已發布"

  install_from_archive "$tmp_archive"
  rm -f "$tmp_archive"
  log "執行 \`dbc doctor\` 檢查環境是否就緒。"
}

# 用 BASH_SOURCE/$0 比對來判斷「有沒有被 source」在 curl | bash（腳本
# 從 stdin 餵進 bash，沒有真的檔案）情況下會失效：BASH_SOURCE 是空的，
# 跟 $0（這時候是 "bash"）永遠對不上，導致 main 永遠不會被呼叫，腳本
# 抓下來卻什麼都沒發生。改用 (return 0 2>/dev/null) 判斷，這個寫法在
# 直接執行、curl | bash、source 三種情況下都正確。
(return 0 2>/dev/null) || main "$@"
