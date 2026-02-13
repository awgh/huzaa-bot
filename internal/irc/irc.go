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
