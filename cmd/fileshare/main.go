package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/awgh/huzaa-bot/internal/config"
	"github.com/awgh/huzaa-bot/internal/fileshare"
	"github.com/awgh/huzaa-bot/internal/irc"
	"github.com/awgh/huzaa-bot/internal/turnclient"
	ircgo "github.com/fluffle/goirc/client"
)

func main() {
	confDir := flag.String("confdir", "config", "Config directory with *.json")
	flag.Parse()

	configs, err := config.LoadFileshareConfigs(*confDir)
	if err != nil {
		log.Fatalf("load configs: %v", err)
	}
	if len(configs) == 0 {
		log.Fatal("no valid fileshare configs found")
	}

	root, err := fileshare.ResolveRoot(configs[0].SharedDir)
	if err != nil {
		log.Fatalf("shared dir: %v", err)
	}

	relayClient, err := turnclient.NewClient(configs[0].RelayTURNURL, nil)
	if err != nil {
		log.Fatalf("relay client: %v", err)
	}

	ircCfg := &irc.Config{
		Host:         configs[0].Host,
		Port:         configs[0].Port,
		Nick:         configs[0].Nick,
		Password:     configs[0].Password,
		Channel:      configs[0].Channel,
		Name:         configs[0].Name,
		Version:      configs[0].Version,
		Quit:         configs[0].Quit,
		ProxyEnabled: configs[0].ProxyEnabled,
		Proxy:        configs[0].Proxy,
		SASL:         configs[0].SASL,
	}
	maxUpload := configs[0].MaxUploadBytes
	maxFile := configs[0].MaxFileBytes
	if maxFile == 0 {
		maxFile = 100 * 1024 * 1024 // 100MB
	}

	conn := irc.Connect(ircCfg)
	irc.JoinChannel(conn, configs[0].Channel)

	conn.HandleFunc(ircgo.PRIVMSG, func(c *ircgo.Conn, line *ircgo.Line) {
		msg := line.Args[1]
		replyTo := line.Nick
		isChannel := line.Public()
		send := func(m string) {
			if isChannel {
				c.Notice(configs[0].Channel, m)
			} else {
				c.Privmsg(replyTo, m)
			}
		}

		if cmd, rest, ok := irc.ParseCTCP(msg); ok && irc.IsDCCSSEND(cmd, rest) {
			filename, _, port, ok := irc.ParseDCCSSEND(rest)
			if !ok {
				send("Invalid DCC SSEND.")
				return
			}
			_ = filename
			_ = port
			send("To upload, use .upload first; I'll give you the relay address.")
			return
		}

		parts := strings.Fields(msg)
		if len(parts) == 0 {
			return
		}
		switch parts[0] {
		case ".list", ".ls":
			pattern := ""
			if len(parts) > 1 {
				pattern = parts[1]
			}
			entries, err := fileshare.ListDir(root, pattern)
			if err != nil {
				send("List error: " + err.Error())
				return
			}
			if len(entries) == 0 {
				send("No files.")
				return
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			send(strings.Join(names, ", "))
		case ".get":
			if len(parts) < 2 {
				send("Usage: .get <filename>")
				return
			}
			filename := parts[1]
			safePath, err := fileshare.SafePath(root, filename)
			if err != nil {
				send("Invalid path.")
				return
			}
			f, err := os.Open(safePath)
			if err != nil {
				send("File not found.")
				return
			}
			info, _ := f.Stat()
			if info.IsDir() {
				f.Close()
				send("Not a file.")
				return
			}
			size := info.Size()
			if maxFile > 0 && size > maxFile {
				f.Close()
				send("File too large.")
				return
			}
			sessionID, err := turnclient.GenerateSessionID()
			if err != nil {
				f.Close()
				send("Error creating session.")
				return
			}
			host, port, sess, err := relayClient.RegisterDownload(sessionID, filepath.Base(filename))
			if err != nil {
				f.Close()
				send("Relay error: " + err.Error())
				return
			}
			ctcpMsg := "\x01DCC SSEND " + filepath.Base(filename) + " " + host + " " + strconv.Itoa(port) + " " + strconv.FormatInt(size, 10) + "\x01"
			c.Privmsg(replyTo, ctcpMsg)
			go func() {
				defer f.Close()
				defer sess.Close()
				if err := sess.SendFile(f, maxFile); err != nil {
					log.Printf("send file: %v", err)
				}
			}()
			send("DCC SSEND sent; accept in your client to download from relay.")
		case ".upload":
			sessionID, err := turnclient.GenerateSessionID()
			if err != nil {
				send("Error creating session.")
				return
			}
			filename := "upload"
			if len(parts) > 1 {
				filename = parts[1]
			}
			filename = filepath.Base(filename)
			if filename == "" || filename == "." {
				filename = "upload"
			}
			host, port, stream, err := relayClient.RegisterUploadStream(sessionID, filename)
			if err != nil {
				send("Relay error: " + err.Error())
				return
			}
			safePath, err := fileshare.SafePath(root, filename)
			if err != nil {
				stream.Close()
				send("Invalid filename.")
				return
			}
			go func() {
				defer stream.Close()
				f, err := os.Create(safePath)
				if err != nil {
					send("Could not create file.")
					return
				}
				var r io.Reader = stream
				if maxUpload > 0 {
					r = io.LimitReader(stream, maxUpload)
				}
				_, err = io.Copy(f, r)
				f.Close()
				if err != nil {
					log.Printf("upload write: %v", err)
				}
			}()
			ctcpUpload := "\x01DCC SSEND " + filename + " " + host + " " + strconv.Itoa(port) + "\x01"
			c.Privmsg(replyTo, ctcpUpload)
			send("Accept the DCC above to upload as " + filename + ".")
		case ".help":
			send(".list [pattern] - list files | .get <file> - download | .upload [filename] - get upload address")
		default:
			// ignore
		}
	})

	conn.HandleFunc(ircgo.DISCONNECTED, func(c *ircgo.Conn, l *ircgo.Line) {
		log.Println("Disconnected")
	})

	for {
		if !conn.Connected() {
			log.Println("Connecting...")
			if err := conn.Connect(); err != nil {
				log.Println("Connect:", err)
			}
		}
		time.Sleep(15 * time.Second)
	}
}
