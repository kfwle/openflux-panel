#!/bin/bash
# OpenFlux Panel — удаление всего, что поставил setup.sh:
# systemd-юнит, каталог установки, правила фаервола.
#   curl -fsSL https://raw.githubusercontent.com/kfwle/openflux-panel/main/uninstall.sh | bash
# или локально:  bash uninstall.sh [--dir /opt/openflux-panel] [--yes] [--keep-fw]
#
# Go, git и своп НЕ трогаем (общие для системы). Данные ключей лежат
# внутри каталога установки и удаляются вместе с ним.

set -u

INSTALL_DIR="${INSTALL_DIR:-/opt/openflux-panel}"
ASSUME_YES=0
KEEP_FW=0

say()  { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[!]\033[0m %s\n' "$*"; }
die()  { printf '\033[1;31m[✘]\033[0m %s\n' "$*"; exit 1; }

ask_yesno() { # ask_yesno <prompt> <default>
  local prompt="$1" def="$2" val=""
  if [ "$ASSUME_YES" = "1" ]; then return 0; fi
  if [ -t 0 ]; then printf '%s [%s]: ' "$prompt" "$def" >&2; read -r val || true
  elif [ -e /dev/tty ]; then printf '%s [%s]: ' "$prompt" "$def" >/dev/tty; read -r val </dev/tty || true
  else echo "$def"; return 0; fi
  [ -z "$val" ] && val="$def"
  case "$val" in y|Y|yes|YES|д|Д) return 0;; *) return 1;; esac
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dir) INSTALL_DIR="$2"; shift 2;;
    --yes) ASSUME_YES=1; shift;;
    --keep-fw) KEEP_FW=1; shift;;
    *) die "Неизвестный флаг: $1";;
  esac
done

echo "——— OpenFlux Panel · удаление ———"
say "Каталог: $INSTALL_DIR"

if [ ! -d "$INSTALL_DIR/panel" ]; then
  warn "Каталог $INSTALL_DIR/panel не найден — ставить было нечего (или другой --dir)."
fi
if ! ask_yesno "Удалить панель, все ключи и данные БЕЗВОЗВРАТНО" "no"; then
  echo "Отмена."; exit 0
fi

# ---------- 1. systemd ----------
if [ -f /etc/systemd/system/openflux-panel.service ]; then
  say "Останавливаю и удаляю сервис…"
  systemctl stop openflux-panel 2>/dev/null || true
  systemctl disable openflux-panel 2>/dev/null || true
  rm -f /etc/systemd/system/openflux-panel.service
  systemctl daemon-reload 2>/dev/null || true
else
  warn "systemd-юнит не найден — пропускаю."
fi

# добиваем висящие процессы панели (точное имя бинаря; скрипт себя не заденет)
if pgrep -x "openflux-panel" >/dev/null 2>&1; then
  say "Глушу оставшиеся процессы панели…"
  pkill -x "openflux-panel" 2>/dev/null || true
  sleep 2
fi

# ---------- 1b. reverse-proxy (только наш managed-блок/файл; серты не трогаем) ----------
if [ "$(id -u)" = "0" ]; then
  if [ -f /etc/nginx/sites-available/openflux-panel ] || [ -f /etc/nginx/conf.d/openflux-panel.conf ]; then
    say "Убираю конфиг nginx…"
    rm -f /etc/nginx/sites-available/openflux-panel /etc/nginx/sites-enabled/openflux-panel /etc/nginx/conf.d/openflux-panel.conf
    nginx -t >/dev/null 2>&1 && (systemctl reload nginx 2>/dev/null || nginx -s reload 2>/dev/null || true)
  fi
  if [ -f /etc/caddy/Caddyfile ] && grep -q "openflux-panel managed" /etc/caddy/Caddyfile 2>/dev/null; then
    say "Убираю блок из Caddyfile…"
    sed -i '/# --- openflux-panel managed ---/,/# --- end openflux-panel ---/d' /etc/caddy/Caddyfile
    systemctl reload caddy 2>/dev/null || systemctl restart caddy 2>/dev/null || true
  fi
fi

# ---------- 2. порты из настроек (для чистки фаервола) ----------
PANEL_PORT=""
D_FROM=""; D_TO=""
UNIT="/etc/systemd/system/openflux-panel.service"
if [ -f "$UNIT" ]; then
  PANEL_PORT=$(grep -o '\-port [0-9]*' "$UNIT" | awk '{print $2}')
fi
PJSON="$INSTALL_DIR/panel/data/panel.json"
if [ -f "$PJSON" ]; then
  D_FROM=$(grep -o '"direct_from": *[0-9]*' "$PJSON" | grep -o '[0-9]*')
  D_TO=$(grep -o '"direct_to": *[0-9]*' "$PJSON" | grep -o '[0-9]*')
fi
[ -z "$PANEL_PORT" ] && PANEL_PORT=8080
[ -z "$D_FROM" ] && D_FROM=20000
[ -z "$D_TO" ] && D_TO=20099

# ---------- 3. фаервол ----------
if [ "$KEEP_FW" = "0" ] && [ "$(id -u)" = "0" ]; then
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
    say "Чищу ufw: $PANEL_PORT, $D_FROM:$D_TO…"
    ufw delete allow "$PANEL_PORT"/tcp >/dev/null 2>&1 || true
    ufw delete allow "$D_FROM:$D_TO"/tcp >/dev/null 2>&1 || true
  elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    say "Чищу firewalld…"
    firewall-cmd --permanent --remove-port="$PANEL_PORT"/tcp >/dev/null 2>&1 || true
    firewall-cmd --permanent --remove-port="$D_FROM-$D_TO"/tcp >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
  elif command -v iptables >/dev/null 2>&1; then
    say "Чищу iptables…"
    iptables -D INPUT -p tcp --dport "$PANEL_PORT" -j ACCEPT 2>/dev/null || true
    iptables -D INPUT -p tcp --dport "$D_FROM:$D_TO" -j ACCEPT 2>/dev/null || true
  else
    warn "Фаервол не найден — чистить нечего."
  fi
  # NOTE: правило l3 против kernel RST (OUTPUT … -j DROP) не трогаем:
  # оно привязано к egress-IP, а не к панели. Убрать вручную при желании:
  # iptables -D OUTPUT -p tcp --tcp-flags RST RST -s <egress-ip> -j DROP
fi

# ---------- 4. каталог ----------
if [ -d "$INSTALL_DIR" ]; then
  say "Удаляю $INSTALL_DIR …"
  rm -rf "$INSTALL_DIR"
fi
rm -f /tmp/SHA256SUMS-core

echo ""
echo "——— Готово: панель, сервис, ключи, данные и правила фаервола удалены ———"
echo "Остались нетронутыми: Go, git, своп. Правило l3 RST (если было) — см. выше."
