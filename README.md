# Huzaa bot

IRC bot that provides DCC file sharing via a relay. Uses the same relay protocol as [huzaa-relay](https://github.com/awgh/huzaa-relay); keep both in sync if the protocol changes.

## Build

```bash
go build -o fileshare ./cmd/fileshare
```

## Config

Copy `config/fileshare.json.sample` to `config/fileshare.json` (or add JSON files to the config directory). Required: `Host`, `SharedDir`, `RelayTURNURL`. Set `RelayAuthUsername` and `RelayAuthSecret` to match one of the relay's `turn_users` entries (auth is required; empty username is not supported). Optional: `MaxUploadBytes`, `MaxFileBytes` (default 100MB for downloads).

## Run

```bash
./fileshare -confdir config
```

## Commands

All commands are accepted by **private message only** (not in channel). Direction is from the user’s perspective:

- `.list [pattern]` – list files
- `.download <file>` – get a file (empty files rejected)
- `.put` / `.upload [filename]` – send a file (default name: `upload-YYYYMMDD-HHMMSS` if omitted)
- `.help` – show commands (one short line)

**DCC SSEND and clients:** The bot sends the relay’s IP in dotted-decimal form in the DCC line so clients that expect a numeric host (e.g. KVIrc) recognize it. Download uses DCC SSEND (bot sends to you); upload uses DCC SRECV (you send to bot). You need a client that supports both (e.g. KVIrc with SSL). Accept SSEND to download, SRECV to upload in the DCC window.

**DCC RESUME:** Interrupted downloads can be resumed. When the client sends DCC RESUME (filename, port, position), the bot replies with DCC ACCEPT and a new port; the client connects there and receives data from the given byte position to end of file.

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
   - **`RelayAuthUsername`** and **`RelayAuthSecret`** – Required; must match one of the relay's `turn_users` entries.
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
