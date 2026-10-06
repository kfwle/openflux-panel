#!/bin/bash
# install.sh — installs OpenFlux Panel on a Linux VPS (same host as openflux exit).
set -e
APP_DIR="/opt/openflux-panel"
PORT="${PANEL_PORT:-8080}"

echo "== OpenFlux Panel installer =="
if ! command -v go >/dev/null 2>&1; then
  echo "Go not found, installing Go 1.22..."
  wget -q https://go.dev/dl/go1.22.5.linux-amd64.tar.gz -O /tmp/go.tgz
  rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz
  export PATH=$PATH:/usr/local/go/bin
fi

mkdir -p "$APP_DIR"
cp -r . "$APP_DIR/" 2>/dev/null || true
cd "$APP_DIR"
go build -o openflux-panel .
chmod +x openflux-panel

# data dir + defaults from env
mkdir -p "$APP_DIR/data"
export PANEL_DATA="$APP_DIR/data" PANEL_PORT="$PORT"

# systemd unit
cat > /etc/systemd/system/openflux-panel.service <<EOF
[Unit]
Description=OpenFlux Panel
After=network.target

[Service]
Type=simple
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/openflux-panel -port $PORT -data $APP_DIR/data
Restart=always
RestartSec=3
Environment=ADMIN_USER=${ADMIN_USER:-admin}
Environment=ADMIN_PASS=${ADMIN_PASS:-admin123}

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now openflux-panel
sleep 2
IP=$(hostname -I 2>/dev/null | awk '{print $1}')
echo ""
echo "Done! Panel: http://${IP:-SERVER_IP}:$PORT"
echo "Login: ${ADMIN_USER:-admin} / ${ADMIN_PASS:-admin123}  (change password in Settings!)"
echo "Next steps:"
echo "  1. Put 'openflux' binary next to panel or set path in Settings."
echo "  2. Set Share-host to this server's public IP."
echo "  3. Open direct ports range in firewall (default 20000-21000 TCP)."
echo "  4. For l3 mode: run as root + iptables RST rule (see README)."
