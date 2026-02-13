# Huzaa bot

IRC bot that provides DCC file sharing via a relay. Uses the same relay protocol as [huzaa-relay](https://github.com/awgh/huzaa-relay); keep both in sync if the protocol changes.

## Build

```bash
go build -o fileshare ./cmd/fileshare
```

## Config

Copy `config/fileshare.json.sample` to `config/fileshare.json` (or add JSON files to the config directory). Required: `Host`, `SharedDir`, `RelayTURNURL`. Optional: `MaxUploadBytes`, `MaxFileBytes` (default 100MB for downloads).

## Run

```bash
./fileshare -confdir config
```

## Commands

- `.list [pattern]` – list files in the shared directory
- `.get <filename>` – send DCC SSEND (user accepts to download from relay)
- `.upload [filename]` – send DCC SSEND so user can upload to the shared dir
- `.help` – show commands

## Deploy on IONOS VPS

The script `install-bot.sh` installs the bot on a Debian VPS (e.g. IONOS) with a systemd unit and config template for localhost IRC plus relay URL.

**Prereqs**

- IRC server (e.g. Ergo) and huzaa-relay already running. Relay reachable at `turns://RELAY_HOST:5349`.
- On the same host as Ergo: run `install-relay.sh` first, then this script.

**Install (as root)**

```bash
scp install-bot.sh root@irc.example.com:/root/
ssh root@irc.example.com 'bash /root/install-bot.sh'
```

Use a different relay host if the relay is not on the same server:

```bash
RELAY_HOST=relay.example.com bash /root/install-bot.sh
```

**Manual setup (required before starting)**

1. **Register an IRC account for the bot**  
   Connect to your IRC server, register a nick (e.g. `HuzaaBot`) with NickServ, and note the password.

2. **Edit config**  
   `sudo nano /opt/huzaa-bot/config/fileshare.json`  
   Set at least:
   - **`Password`** – For Ergo use `Nick:password` (e.g. `HuzaaBot:YourSecureBotPassword`).
   - **`Channel`** – Channel to join (e.g. `#files`).
   - **`RelayTURNURL`** – Must match your relay (e.g. `turns://irc.example.com:5349`). The install script sets this from `RELAY_HOST`; change if needed.
   - **`SharedDir`** – Install script sets `/opt/huzaa-bot/shared`; ensure it exists and is writable by `huzaa-bot`.

3. **Start the bot**

   ```bash
   sudo systemctl start huzaa-bot
   sudo systemctl status huzaa-bot
   journalctl -u huzaa-bot -f
   ```

**Useful commands**

| Action        | Command                          |
|---------------|-----------------------------------|
| Start         | `sudo systemctl start huzaa-bot`  |
| Stop          | `sudo systemctl stop huzaa-bot`   |
| Restart       | `sudo systemctl restart huzaa-bot`|
| Logs          | `journalctl -u huzaa-bot -f`      |
| Config        | `/opt/huzaa-bot/config/fileshare.json` |
