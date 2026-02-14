package irc

import (
	"crypto/tls"
	"strconv"
	"strings"
	"time"

	irc "github.com/fluffle/goirc/client"
)

// Config holds IRC connection config (Marvin-style).
type Config struct {
	Host         string
	Port         string
	Nick         string
	Password     string
	Channel      string
	Name         string
	Version      string
	Quit         string
	ProxyEnabled bool
	Proxy        string
	SASL         bool
}

// Connect creates and connects an IRC client. Caller must call conn.Connect() and set handlers.
func Connect(cfg *Config) *irc.Conn {
	ircCfg := irc.NewConfig(cfg.Nick)
	ircCfg.SSL = true
	ircCfg.SSLConfig = &tls.Config{ServerName: cfg.Host, InsecureSkipVerify: true}
	ircCfg.Server = cfg.Host + ":" + cfg.Port
	ircCfg.Me.Ident = cfg.Nick
	ircCfg.Me.Name = cfg.Name
	ircCfg.Pass = cfg.Password
	ircCfg.Version = cfg.Version
	ircCfg.QuitMessage = cfg.Quit
	ircCfg.PingFreq = 120 * time.Second
	if cfg.ProxyEnabled && cfg.Proxy != "" {
		ircCfg.Proxy = cfg.Proxy
	}
	conn := irc.Client(ircCfg)
	conn.EnableStateTracking()
	return conn
}

// JoinChannel joins the configured channel on CONNECTED.
func JoinChannel(conn *irc.Conn, channel string) {
	conn.HandleFunc(irc.CONNECTED, func(c *irc.Conn, l *irc.Line) {
		go func() {
			time.Sleep(time.Second)
			c.Join(channel)
		}()
	})
}

// ParseCTCP extracts CTCP payload from a PRIVMSG (message between \x01 and \x01).
func ParseCTCP(msg string) (cmd string, rest string, ok bool) {
	if len(msg) < 2 || msg[0] != '\x01' || msg[len(msg)-1] != '\x01' {
		return "", "", false
	}
	inner := msg[1 : len(msg)-1]
	parts := strings.SplitN(inner, " ", 2)
	cmd = strings.ToUpper(parts[0])
	if len(parts) > 1 {
		rest = parts[1]
	}
	return cmd, rest, true
}

// IsDCCSSEND returns true if the CTCP is DCC SSEND (filename host port [size]).
func IsDCCSSEND(cmd, rest string) bool {
	return cmd == "DCC" && strings.HasPrefix(rest, "SSEND ")
}

// ParseDCCSSEND parses "SSEND filename host port" or "SSEND filename host port size".
func ParseDCCSSEND(rest string) (filename, host string, port int, ok bool) {
	if !strings.HasPrefix(rest, "SSEND ") {
		return "", "", 0, false
	}
	rest = rest[6:]
	parts := strings.Split(rest, " ")
	if len(parts) < 4 {
		return "", "", 0, false
	}
	filename = parts[0]
	host = parts[1]
	p, err := strconv.Atoi(parts[2])
	if err != nil || p <= 0 {
		return "", "", 0, false
	}
	return filename, host, p, true
}

// IsDCCResume returns true if the CTCP is DCC RESUME (filename port position).
func IsDCCResume(cmd, rest string) bool {
	return strings.ToUpper(cmd) == "DCC" && strings.HasPrefix(strings.ToUpper(rest), "RESUME ")
}

// ParseDCCResume parses "RESUME filename port position". Position is the byte offset to resume from.
// The RESUME verb is matched case-insensitively.
func ParseDCCResume(rest string) (filename string, port int, position int64, ok bool) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(strings.ToUpper(rest), "RESUME ") {
		return "", 0, 0, false
	}
	rest = strings.TrimSpace(rest[7:])
	parts := strings.Split(rest, " ")
	if len(parts) < 3 {
		return "", 0, 0, false
	}
	filename = parts[0]
	p, err := strconv.Atoi(parts[1])
	if err != nil || p < 0 {
		return "", 0, 0, false
	}
	pos, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || pos < 0 {
		return "", 0, 0, false
	}
	return filename, p, pos, true
}

// DCCResumeRestFromMessage returns the "RESUME filename port position" part of msg if it is a DCC RESUME
// (either CTCP-wrapped \x01DCC RESUME ...\x01 or plain "DCC RESUME ..."). Used so the bot recognizes
// RESUME even when the client or server omits CTCP delimiters.
func DCCResumeRestFromMessage(msg string) (rest string, ok bool) {
	msg = strings.TrimSpace(msg)
	if len(msg) >= 2 && msg[0] == '\x01' && msg[len(msg)-1] == '\x01' {
		inner := msg[1 : len(msg)-1]
		parts := strings.SplitN(inner, " ", 2)
		if len(parts) == 2 && strings.ToUpper(parts[0]) == "DCC" && strings.HasPrefix(strings.ToUpper(parts[1]), "RESUME ") {
			return parts[1], true
		}
	}
	if strings.HasPrefix(strings.ToUpper(msg), "DCC RESUME ") {
		rest = "RESUME " + strings.TrimSpace(msg[11:]) // so ParseDCCResume receives "RESUME filename port position"
		return rest, true
	}
	return "", false
}

// DCCAcceptCTCP returns the CTCP string for DCC ACCEPT (filename port position).
// Used to allow a client to resume a download from the given position; client then connects to port.
func DCCAcceptCTCP(filename string, port int, position int64) string {
	return "\x01DCC ACCEPT " + filename + " " + strconv.Itoa(port) + " " + strconv.FormatInt(position, 10) + "\x01"
}
