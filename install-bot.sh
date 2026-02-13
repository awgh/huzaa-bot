#!/bin/bash
#
# install-bot.sh — Install huzaa-bot on a Debian VPS (IONOS or similar).
# Run as root on a host that can reach the IRC server and huzaa-relay.
# Typically run on the same host as Ergo and huzaa-relay (after install-relay.sh).
#
# Before starting: edit config and set Password, Channel, SharedDir, RelayTURNURL.
# See README "Deploy on IONOS VPS" for manual steps.
#
if grep -q $'\r' "$0" 2>/dev/null; then
  exec sed 's/\r$//' "$0" | bash -s "$@"
fi
set -e
export DEBIAN_FRONTEND=noninteractive

BOT_HOME="/opt/huzaa-bot"
BOT_USER="huzaa-bot"
CONFIG_NAME="fileshare"
# IRC and relay host (e.g. irc.example.com). Override with RELAY_HOST=your.host install-bot.sh
RELAY_HOST="${RELAY_HOST:-irc.example.com}"
GO_VERSION="1.23.5"

echo "=== 1. Go (if not already installed) ==="
if ! command -v go &>/dev/null; then
  apt-get update -qq
  apt-get install -y -qq wget
  cd /tmp
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
  rm -f "go${GO_VERSION}.linux-amd64.tar.gz"
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/golang.sh
  export PATH="$PATH:/usr/local/go/bin"
fi
export PATH="$PATH:/usr/local/go/bin"

echo "=== 2. Fetch huzaa-bot and build ==="
GOPATH="${GOPATH:-/root/go}"
export GOPATH
mkdir -p "$GOPATH/src/github.com/awgh"
if [[ ! -d "$GOPATH/src/github.com/awgh/huzaa-bot/.git" ]]; then
  apt-get install -y -qq git
  git clone --depth 1 https://github.com/awgh/huzaa-bot.git "$GOPATH/src/github.com/awgh/huzaa-bot"
fi
cd "$GOPATH/src/github.com/awgh/huzaa-bot"
go build -o fileshare_bin ./cmd/fileshare

echo "=== 3. huzaa-bot user and directories ==="
if ! id "$BOT_USER" &>/dev/null; then
  useradd --system --home-dir "$BOT_HOME" --no-create-home --shell /usr/sbin/nologin "$BOT_USER"
fi
mkdir -p "$BOT_HOME/config" "$BOT_HOME/shared"
cp -f fileshare_bin "$BOT_HOME/fileshare"

echo "=== 4. Bot config (IRC localhost + relay URL) ==="
cat > "$BOT_HOME/config/${CONFIG_NAME}.json" << EOF
{
  "Host": "127.0.0.1",
  "Port": "6697",
  "Nick": "HuzaaBot",
  "Password": "HuzaaBot:REPLACE_WITH_BOT_PASSWORD",
  "Channel": "#files",
  "Name": "Huzaa File Bot",
  "Version": "Huzaa 1.0",
  "Quit": "Bye!",
  "ProxyEnabled": false,
  "Proxy": "",
  "SASL": false,
  "SharedDir": "${BOT_HOME}/shared",
  "RelayTURNURL": "turns://${RELAY_HOST}:5349",
  "RelayAuthUsername": "",
  "RelayAuthSecret": "",
  "MaxUploadBytes": 10485760,
  "MaxFileBytes": 104857600
}
EOF
chown -R "$BOT_USER:$BOT_USER" "$BOT_HOME"
chmod 600 "$BOT_HOME/config/${CONFIG_NAME}.json"

echo "=== 5. systemd service ==="
cat > /etc/systemd/system/huzaa-bot.service << EOF
[Unit]
Description=Huzaa IRC file-sharing bot
After=network.target

[Service]
Type=simple
User=${BOT_USER}
Group=${BOT_USER}
WorkingDirectory=${BOT_HOME}
ExecStart=${BOT_HOME}/fileshare -confdir ${BOT_HOME}/config
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable huzaa-bot

echo ""
echo "=== Huzaa bot installed at ${BOT_HOME} ==="
echo "Before starting:"
echo "  1. Register the bot account on IRC (see README)."
echo "  2. Edit ${BOT_HOME}/config/${CONFIG_NAME}.json: set Password, Channel."
echo "  3. RelayTURNURL is set to turns://${RELAY_HOST}:5349 (override with RELAY_HOST=... when running this script)."
echo "  4. Start: systemctl start huzaa-bot"
echo "Logs: journalctl -u huzaa-bot -f"
