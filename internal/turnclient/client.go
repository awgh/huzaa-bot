package turnclient

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/awgh/huzaa-bot/internal/relayprotocol"
)

// Debug enables debug logging for SendFile (chunk and total bytes). Set by main when -debug is true.
var Debug bool

// GenerateSessionID returns a random session ID for relay sessions.
func GenerateSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// Client connects to the relay over TLS and performs session register + stream.
type Client struct {
	relayHost   string
	relayPort   int
	tlsConfig   *tls.Config
	authUsername string
	authSecret   string
}

// NewClient creates a relay client. turnURL is e.g. "turns://irc.example.com:5349".
// username and secret are optional; when both are set, the client sends MsgAuth after each dial (for relays with turn_users).
func NewClient(turnURL string, tlsConfig *tls.Config, username, secret string) (*Client, error) {
	u, err := url.Parse(turnURL)
	if err != nil {
		return nil, err
	}
	port := 5349
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("missing host in %s", turnURL)
	}
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	}
	return &Client{
		relayHost:     host,
		relayPort:     port,
		tlsConfig:     tlsConfig,
		authUsername:  username,
		authSecret:    secret,
	}, nil
}

// auth sends MsgAuth (username + secret) and waits for MsgAuthOk or MsgError. Required for all relay connections.
func (c *Client) auth(conn *tls.Conn) error {
	if c.authUsername == "" || c.authSecret == "" {
		return fmt.Errorf("relay auth: username and secret required")
	}
	un := []byte(c.authUsername)
	payload := make([]byte, 4+len(un)+len(c.authSecret))
	binary.BigEndian.PutUint32(payload[:4], uint32(len(un)))
	copy(payload[4:], un)
	copy(payload[4+len(un):], c.authSecret)
	if err := relayprotocol.WriteFrame(conn, relayprotocol.MsgAuth, payload); err != nil {
		return err
	}
	msgType, resp, err := relayprotocol.ReadFrame(conn)
	if err != nil {
		return err
	}
	if msgType == relayprotocol.MsgError {
		return fmt.Errorf("relay auth: %s", string(resp))
	}
	if msgType != relayprotocol.MsgAuthOk {
		return fmt.Errorf("relay: unexpected response to auth (type %d)", msgType)
	}
	return nil
}

// DownloadSession holds the connection for a download after RegisterDownload.
type DownloadSession struct {
	conn *tls.Conn
}

// SendFile streams the file content to the relay.
func (d *DownloadSession) SendFile(content io.Reader, maxBytes int64) error {
	buf := make([]byte, 32*1024)
	var sent int64
	for {
		n, err := content.Read(buf)
		if n > 0 {
			if maxBytes > 0 && sent+int64(n) > maxBytes {
				n = int(maxBytes - sent)
			}
			// Copy payload so we don't reuse buf before the write is flushed to the network.
			payload := make([]byte, n)
			copy(payload, buf[:n])
			if err := relayprotocol.WriteFrame(d.conn, relayprotocol.MsgData, payload); err != nil {
				return err
			}
			sent += int64(n)
			if Debug {
				log.Printf("[debug] SendFile sent chunk %d bytes, total %d", n, sent)
			}
			if maxBytes > 0 && sent >= maxBytes {
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if Debug {
		log.Printf("[debug] SendFile sending EOF, total %d bytes", sent)
	}
	return relayprotocol.WriteFrame(d.conn, relayprotocol.MsgEOF, nil)
}

// Close closes the session connection.
func (d *DownloadSession) Close() error {
	if d.conn != nil {
		err := d.conn.Close()
		d.conn = nil
		return err
	}
	return nil
}

// RegisterDownload registers a download session and returns the relay host, port, and a session to stream the file.
func (c *Client) RegisterDownload(sessionID, filename string) (host string, port int, sess *DownloadSession, err error) {
	conn, err := c.dial()
	if err != nil {
		return "", 0, nil, err
	}
	if err := c.auth(conn); err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	payload := make([]byte, 0, 36+len(filename))
	if len(sessionID) > 36 {
		sessionID = sessionID[:36]
	}
	payload = append(payload, []byte(sessionID)...)
	for len(payload) < 36 {
		payload = append(payload, 0)
	}
	payload = append(payload, filename...)

	if err := relayprotocol.WriteFrame(conn, relayprotocol.MsgRegisterDownload, payload); err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	msgType, resp, err := relayprotocol.ReadFrame(conn)
	if err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	if msgType == relayprotocol.MsgError {
		conn.Close()
		return "", 0, nil, fmt.Errorf("relay: %s", string(resp))
	}
	if msgType != relayprotocol.MsgPortAlloc || len(resp) < 4 {
		conn.Close()
		return "", 0, nil, fmt.Errorf("relay: unexpected response")
	}
	port = int(binary.BigEndian.Uint32(resp))
	return c.relayHost, port, &DownloadSession{conn: conn}, nil
}

// UploadStream implements io.Reader for upload data from the relay.
type UploadStream struct {
	conn *tls.Conn
	buf  []byte
	eof  bool
}

func (u *UploadStream) Read(p []byte) (n int, err error) {
	for len(u.buf) == 0 && !u.eof {
		msgType, payload, err := relayprotocol.ReadFrame(u.conn)
		if err != nil {
			return 0, err
		}
		if msgType == relayprotocol.MsgEOF {
			u.eof = true
			return 0, io.EOF
		}
		if msgType == relayprotocol.MsgError {
			return 0, fmt.Errorf("relay: %s", string(payload))
		}
		if msgType != relayprotocol.MsgData {
			return 0, fmt.Errorf("relay: unexpected msg type %d", msgType)
		}
		u.buf = payload
	}
	if len(u.buf) == 0 {
		return 0, io.EOF
	}
	n = copy(p, u.buf)
	u.buf = u.buf[n:]
	return n, nil
}

func (u *UploadStream) Close() error {
	if u.conn != nil {
		err := u.conn.Close()
		u.conn = nil
		return err
	}
	return nil
}

// RegisterUploadStream registers upload and returns a stream to read the uploaded file.
func (c *Client) RegisterUploadStream(sessionID, filename string) (host string, port int, stream *UploadStream, err error) {
	conn, err := c.dial()
	if err != nil {
		return "", 0, nil, err
	}
	if err := c.auth(conn); err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	payload := make([]byte, 0, 36+len(filename))
	if len(sessionID) > 36 {
		sessionID = sessionID[:36]
	}
	payload = append(payload, []byte(sessionID)...)
	for len(payload) < 36 {
		payload = append(payload, 0)
	}
	payload = append(payload, filename...)

	if err := relayprotocol.WriteFrame(conn, relayprotocol.MsgRegisterUpload, payload); err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	msgType, resp, err := relayprotocol.ReadFrame(conn)
	if err != nil {
		conn.Close()
		return "", 0, nil, err
	}
	if msgType == relayprotocol.MsgError {
		conn.Close()
		return "", 0, nil, fmt.Errorf("relay: %s", string(resp))
	}
	if msgType != relayprotocol.MsgPortAlloc || len(resp) < 4 {
		conn.Close()
		return "", 0, nil, fmt.Errorf("relay: unexpected response")
	}
	port = int(binary.BigEndian.Uint32(resp))
	return c.relayHost, port, &UploadStream{conn: conn}, nil
}

func (c *Client) dial() (*tls.Conn, error) {
	addr := fmt.Sprintf("%s:%d", c.relayHost, c.relayPort)
	tcpConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(tcpConn, c.tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		tcpConn.Close()
		return nil, err
	}
	return tlsConn, nil
}
