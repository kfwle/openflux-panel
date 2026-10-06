#!/bin/bash
# OpenFlux Panel — one-line installer.
#   curl -fsSL https://your-host/setup.sh | bash
# Non-interactive too:
#   curl -fsSL https://your-host/setup.sh | bash -s -- --port 8080 --user admin --mode l4 --yes
#
# What it does: installs Go (if needed), clones panel + OpenFlux core,
# builds both, writes systemd unit, opens firewall, starts the panel.

set -u

# ---------- configurable defaults (override via env or flags) ----------
PANEL_REPO="${PANEL_REPO:-https://github.com/kfwle/openflux-panel.git}"
CORE_REPO="${CORE_REPO:-https://github.com/p1neappleXpress/OpenFlux}"
INSTALL_DIR="${INSTALL_DIR:-/opt/openflux-panel}"
PANEL_PORT="${PANEL_PORT:-8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-}"          # empty -> random generated
SHARE_HOST="${SHARE_HOST:-}"          # empty -> autodetect, then ask
DIRECT_FROM="${DIRECT_FROM:-20000}"
DIRECT_TO="${DIRECT_TO:-20099}"
DEFAULT_MODE="${DEFAULT_MODE:-}"      # empty -> l3 on root+linux, else l4
BUILD_CORE="${BUILD_CORE:-ask}"       # yes/no/ask
CORE_TAG="${CORE_TAG:-}"              # pin node release (node-vX.Y.Z); empty -> latest
OPEN_FW="${OPEN_FW:-ask}"             # yes/no/ask
ASSUME_YES=0

# ---------- helpers ----------
say()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[✘]\033[0m %s\n' "$*"; exit 1; }

# read from TTY so `curl | bash` keeps working
ask() { # ask <var> <prompt> <default>
  local var="$1" prompt="$2" def="$3" val=""
  if [ "$ASSUME_YES" = "1" ]; then eval "$var=\"\$def\""; return; fi
  if [ -t 0 ]; then printf '%s [%s]: ' "$prompt" "$def" >&2; read -r val || true
  elif [ -e /dev/tty ]; then printf '%s [%s]: ' "$prompt" "$def" >/dev/tty; read -r val </dev/tty || true
  else eval "$var=\"\$def\""; return; fi
  [ -z "$val" ] && val="$def"
  eval "$var=\"\$val\""
}

ask_yesno() { # ask_yesno <var> <prompt> <defaultY/N>
  local var="$1" prompt="$2" def="$3" val=""
  if [ "$var" = "DO_CORE" ] && [ "$BUILD_CORE" != "ask" ]; then eval "$var=\"$BUILD_CORE\""; return; fi
  if [ "$var" = "DO_FW" ] && [ "$OPEN_FW" != "ask" ]; then eval "$var=\"$OPEN_FW\""; return; fi
  if [ "$ASSUME_YES" = "1" ]; then eval "$var=\"yes\""; return; fi
  if [ -t 0 ]; then printf '%s [%s]: ' "$prompt" "$def" >&2; read -r val || true
  elif [ -e /dev/tty ]; then printf '%s [%s]: ' "$prompt" "$def" >/dev/tty; read -r val </dev/tty || true
  else eval "$var=\"\$def\""; return; fi
  [ -z "$val" ] && val="$def"
  case "$val" in y|Y|yes|YES|д|Д|da) eval "$var=yes";; *) eval "$var=no";; esac
}

randpass() { tr -dc 'A-Za-z0-9' </dev/urandom 2>/dev/null | head -c 14; echo; }

detect_ip() {
  local ip=""
  ip=$(curl -fsSL --max-time 8 https://api.ipify.org 2>/dev/null || true)
  [ -n "$ip" ] || ip=$(hostname -I 2>/dev/null | awk '{print $1}')
  printf '%s' "$ip"
}

need_root() {
  if [ "$(id -u)" != "0" ]; then
    warn "Запущено не от root: systemd, фаервол и l3-режим будут недоступны."
    warn "Для полной установки запустите через sudo."
    SUDO=""
    if [ "$DEFAULT_MODE" = "" ]; then DEFAULT_MODE="l4"; fi
  else
    SUDO=""
    if [ "$DEFAULT_MODE" = "" ]; then DEFAULT_MODE="l3"; fi
  fi
}

# ---------- flags ----------
while [ $# -gt 0 ]; do
  case "$1" in
    --port) PANEL_PORT="$2"; shift 2;;
    --user) ADMIN_USER="$2"; shift 2;;
    --pass) ADMIN_PASS="$2"; shift 2;;
    --host) SHARE_HOST="$2"; shift 2;;
    --from) DIRECT_FROM="$2"; shift 2;;
    --to) DIRECT_TO="$2"; shift 2;;
    --mode) DEFAULT_MODE="$2"; shift 2;;
    --dir) INSTALL_DIR="$2"; shift 2;;
    --repo) PANEL_REPO="$2"; shift 2;;
    --core-tag) CORE_TAG="$2"; shift 2;;
    --yes) ASSUME_YES=1; shift;;
    --no-core) BUILD_CORE="no"; shift;;
    --no-fw) OPEN_FW="no"; shift;;
    *) die "Неизвестный флаг: $1";;
  esac
done

echo "——— OpenFlux Panel · установка ———"
need_root

# ---------- interactive questions ----------
ask PANEL_PORT "Порт панели" "$PANEL_PORT"
ask ADMIN_USER "Логин админа" "$ADMIN_USER"
if [ -z "$ADMIN_PASS" ]; then
  if [ "$ASSUME_YES" = "1" ]; then ADMIN_PASS="$(randpass)"; GENERATED=1
  else
    ask ADMIN_PASS "Пароль админа (пусто — сгенерировать)" ""
    if [ -z "$ADMIN_PASS" ]; then ADMIN_PASS="$(randpass)"; GENERATED=1; fi
  fi
fi
if [ -z "$SHARE_HOST" ]; then
  DETECTED="$(detect_ip)"
  ask SHARE_HOST "Белый IP сервера (share-host для direct)" "$DETECTED"
fi
ask DIRECT_FROM "Direct-порты: от" "$DIRECT_FROM"
ask DIRECT_TO "Direct-порты: до" "$DIRECT_TO"
if [ "$DEFAULT_MODE" != "l3" ] && [ "$DEFAULT_MODE" != "l4" ]; then
  ask DEFAULT_MODE "Режим exit по умолчанию (l3 = Linux+root быстрее, l4 = везде)" "$DEFAULT_MODE"
fi
ask INSTALL_DIR "Куда ставить" "$INSTALL_DIR"
ask_yesno DO_CORE "Поставить ядро openflux (готовая сборка, иначе компиляция)" "yes"
ask_yesno DO_FW "Открыть порты в фаерволе (панель + direct-диапазон)" "yes"

echo ""
say "Порт панели:      $PANEL_PORT"
say "Админ:             $ADMIN_USER"
say "Share-host:        $SHARE_HOST"
say "Direct-порты:      $DIRECT_FROM–$DIRECT_TO"
say "Режим:             $DEFAULT_MODE"
say "Каталог:           $INSTALL_DIR"
echo ""

# ---------- deps ----------
if ! command -v git >/dev/null 2>&1; then
  say "Ставлю git + curl…"
  $SUDO apt-get update -qq && $SUDO apt-get install -y -qq git curl >/dev/null || \
  $SUDO yum install -y -q git curl >/dev/null || die "Поставьте git и curl вручную"
fi

if ! command -v go >/dev/null 2>&1 || ! go version 2>/dev/null | grep -Eq 'go1\.(2[2-9]|[3-9][0-9])'; then
  say "Ставлю Go 1.22…"
  curl -fsSL https://go.dev/dl/go1.22.5.linux-amd64.tar.gz -o /tmp/go.tgz || die "Не скачался Go"
  $SUDO rm -rf /usr/local/go && $SUDO tar -C /usr/local -xzf /tmp/go.tgz
  export PATH="$PATH:/usr/local/go/bin"
fi
say "Go: $(go version)"

# ---------- sources ----------
$SUDO mkdir -p "$INSTALL_DIR"
$SUDO chown -R "$(id -u):$(id -g)" "$INSTALL_DIR" 2>/dev/null || true
if [ -d "$INSTALL_DIR/panel/.git" ]; then
  say "Обновляю панель…"
  git -C "$INSTALL_DIR/panel" pull --ff-only 2>/dev/null || warn "git pull не удался, использую как есть"
else
  say "Клонирую панель…"
  git clone "$PANEL_REPO" "$INSTALL_DIR/panel" || die "Не клонируется $PANEL_REPO"
fi

# ---------- prebuilt core (no compile, no Go, no gVisor download) ----------
fetch_prebuilt_core() {
  local arch tag base
  case "$(uname -m)" in
    x86_64) arch="amd64";; aarch64|arm64) arch="arm64";; armv7l|armhf) arch="arm";; *)
      warn "Архитектура $(uname -m): готовой сборки нет"; return 1;;
  esac
  if [ -n "$CORE_TAG" ]; then
    tag="$CORE_TAG"
  else
    tag=$(curl -fsSL --max-time 20 "https://api.github.com/repos/p1neappleXpress/OpenFlux/releases?per_page=30" 2>/dev/null \
      | grep -o '"tag_name": *"node-v[^"]*"' | head -1 | sed 's/.*: *"//;s/"//')
    [ -n "$tag" ] || { warn "Не узнал свежий node-релиз"; return 1; }
  fi
  base="https://github.com/p1neappleXpress/OpenFlux/releases/download/$tag"
  say "Качаю готовое ядро $tag (linux-$arch, ~14 МБ)…"
  curl -fsSL --max-time 180 -o "$INSTALL_DIR/panel/openflux-linux-$arch" "$base/openflux-linux-$arch" || return 1
  if curl -fsSL --max-time 60 -o /tmp/SHA256SUMS-core "$base/SHA256SUMS" 2>/dev/null; then
    if (cd "$INSTALL_DIR/panel" && grep "openflux-linux-$arch" /tmp/SHA256SUMS-core | sha256sum -c - >/dev/null 2>&1); then
      say "SHA256 сошёлся."
    else
      warn "SHA256 не сошёлся — бинарь удалён"
      rm -f "$INSTALL_DIR/panel/openflux-linux-$arch"
      return 1
    fi
  else
    warn "SHA256SUMS не скачался — ставлю без проверки"
  fi
  mv "$INSTALL_DIR/panel/openflux-linux-$arch" "$INSTALL_DIR/panel/openflux"
  chmod +x "$INSTALL_DIR/panel/openflux"
}

# ---------- build panel ----------
say "Собираю панель…"
(cd "$INSTALL_DIR/panel" && go build -o openflux-panel .) || die "Сборка панели упала"
chmod +x "$INSTALL_DIR/panel/openflux-panel"

# ---------- core: prebuilt first, source fallback ----------
if [ "$DO_CORE" = "yes" ]; then
  if ! fetch_prebuilt_core; then
    warn "Готовый бинарь не встал — собираю ядро из исходников (долго)…"
  if [ -d "$INSTALL_DIR/core/.git" ]; then
    say "Обновляю ядро…"
    git -C "$INSTALL_DIR/core" pull --ff-only 2>/dev/null || true
  else
    say "Клонирую ядро…"
    git clone --depth 1 "$CORE_REPO" "$INSTALL_DIR/core" || die "Не клонируется $CORE_REPO"
  fi
  say "Собираю ядро (долго, качается gVisor)…"
  (cd "$INSTALL_DIR/core" && CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o openflux .) || die "Сборка ядра упала"
  chmod +x "$INSTALL_DIR/core/openflux"
  ln -sf "$INSTALL_DIR/core/openflux" "$INSTALL_DIR/panel/openflux"
  fi
  CORE_BIN="$INSTALL_DIR/panel/openflux"
else
  CORE_BIN="$INSTALL_DIR/panel/openflux"
  warn "Ядро пропущено: положите бинарь openflux в $INSTALL_DIR/panel/ вручную."
fi

mkdir -p "$INSTALL_DIR/panel/data"

# ---------- firewall ----------
if [ "$DO_FW" = "yes" ] && [ "$(id -u)" = "0" ]; then
  if command -v ufw >/dev/null 2>&1; then
    say "Открываю ufw: $PANEL_PORT, $DIRECT_FROM:$DIRECT_TO…"
    ufw allow "$PANEL_PORT"/tcp >/dev/null 2>&1 || true
    ufw allow "$DIRECT_FROM:$DIRECT_TO"/tcp >/dev/null 2>&1 || true
  elif command -v firewall-cmd >/dev/null 2>&1; then
    say "Открываю firewalld…"
    firewall-cmd --permanent --add-port="$PANEL_PORT"/tcp >/dev/null 2>&1 || true
    firewall-cmd --permanent --add-port="$DIRECT_FROM-$DIRECT_TO"/tcp >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
  elif command -v iptables >/dev/null 2>&1; then
    say "Открываю iptables…"
    iptables -C INPUT -p tcp --dport "$PANEL_PORT" -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport "$PANEL_PORT" -j ACCEPT
    iptables -C INPUT -p tcp --dport "$DIRECT_FROM:$DIRECT_TO" -j ACCEPT 2>/dev/null || iptables -I INPUT -p tcp --dport "$DIRECT_FROM:$DIRECT_TO" -j ACCEPT
  else
    warn "Фаервол не найден — откройте порты вручную."
  fi
  if [ "$DEFAULT_MODE" = "l3" ]; then
    EGRESS="$(ip route get 8.8.8.8 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}')"
    if [ -n "$EGRESS" ]; then
      say "l3: добавляю iptables-правило против kernel RST (egress $EGRESS)…"
      iptables -C OUTPUT -p tcp --tcp-flags RST RST -s "$EGRESS" -j DROP 2>/dev/null || \
      iptables -A OUTPUT -p tcp --tcp-flags RST RST -s "$EGRESS" -j DROP
    fi
  fi
fi

# ---------- seed settings ----------
SETTINGS_JSON="$INSTALL_DIR/panel/data/panel.json"
if [ ! -f "$SETTINGS_JSON" ]; then
  say "Записываю стартовые настройки…"
  cat > "$SETTINGS_JSON" <<EOF
{
  "admin_user": "",
  "pass_hash": "",
  "salt": "",
  "settings": {
    "share_host": "$SHARE_HOST",
    "direct_from": $DIRECT_FROM,
    "direct_to": $DIRECT_TO,
    "openflux_bin": "./openflux",
    "default_mode": "$DEFAULT_MODE",
    "default_codec": "batched",
    "auto_disable": true,
    "poll_interval_sec": 10
  },
  "keys": {},
  "history": []
}
EOF
  chmod 600 "$SETTINGS_JSON"
fi

# ---------- systemd ----------
if [ "$(id -u)" = "0" ] && command -v systemctl >/dev/null 2>&1; then
  say "Пишу systemd-юнит…"
  cat > /etc/systemd/system/openflux-panel.service <<EOF
[Unit]
Description=OpenFlux Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR/panel
ExecStart=$INSTALL_DIR/panel/openflux-panel -port $PANEL_PORT -data $INSTALL_DIR/panel/data
Restart=always
RestartSec=3
Environment=ADMIN_USER=$ADMIN_USER
Environment=ADMIN_PASS=$ADMIN_PASS

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now openflux-panel
  sleep 2
  systemctl is-active --quiet openflux-panel && say "Сервис запущен." || die "Сервис не стартовал: journalctl -u openflux-panel -e"
else
  warn "Без root/systemd: запускайте вручную:"
  echo "  cd $INSTALL_DIR/panel && ADMIN_USER='$ADMIN_USER' ADMIN_PASS='$ADMIN_PASS' ./openflux-panel -port $PANEL_PORT -data $INSTALL_DIR/panel/data"
fi

echo ""
echo "——— Готово ———"
echo "Панель:  http://$SHARE_HOST:$PANEL_PORT"
echo "Логин:   $ADMIN_USER"
echo "Пароль:  $ADMIN_PASS"
[ "${GENERATED:-0}" = "1" ] && echo "(пароль сгенерирован — смените в Настройках)"
echo ""
echo "Дальше: откройте панель → Настройки → проверьте share-host → создайте ключ →"
echo "отсканируйте QR в OpenFluxAndroid."
