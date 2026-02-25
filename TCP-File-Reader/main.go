package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

type Command struct {
	Action string `json:"action"`
	Lines  int    `json:"lines"`
	File   string `json:"file"`
}

func main() {
	port := flag.Int("p", 9001, "port to listen on")
	logPath := flag.String("l", "tcp-file-reader.log", "path to log file")

	flag.Usage = func() {
		printHelp()
	}
	flag.Parse()

	// support 'help' and '--help' as positional tokens
	for _, a := range os.Args[1:] {
		if a == "help" || a == "--help" {
			printHelp()
			return
		}
	}

	// Open log file
	lf, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		return
	}
	defer lf.Close()
	log.SetOutput(lf)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	addr := fmt.Sprintf(":%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	defer ln.Close()
	log.Printf("Listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("accept error:", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	for {
		// Expect a single-line JSON command ending with a newline
		line, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Println("read error:", err)
			}
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var cmd Command
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			writeCRLF(conn, fmt.Sprintf("error: invalid json: %v\n", err))
			continue
		}

		if cmd.Action != "read_file" {
			writeCRLF(conn, fmt.Sprintf("error: unsupported action '%s'\n", cmd.Action))
			continue
		}

		out, err := tailFile(cmd.File, cmd.Lines)
		if err != nil {
			writeCRLF(conn, fmt.Sprintf("error: %v\n", err))
			continue
		}

		// Send the result back to the client (use CRLF line endings)
		if _, err := writeCRLF(conn, out); err != nil {
			log.Println("write error:", err)
			return
		}
	}
}

func tailFile(path string, n int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if n <= 0 {
		// return whole file
		b, err := io.ReadAll(f)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	scanner := bufio.NewScanner(f)
	buf := make([]string, 0, n)
	for scanner.Scan() {
		buf = append(buf, scanner.Text())
		if len(buf) > n {
			buf = buf[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(buf, "\n") + "\n", nil
}

func printHelp() {
	fmt.Println("Usage: tcp-file-reader [-p port] [-l log_file]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -p <port>         Port to listen on (default 9001)")
	fmt.Println("  -l <log_file>     Path to log file (default tcp-file-reader.log)")
	fmt.Println("  help, -h, --help  Show this help")
	fmt.Println()
	fmt.Println("Send a single-line JSON command over TCP, terminated with CRLF, for example:")
	fmt.Println(`  {"action":"read_file","lines":3,"file":"C:\\path\\to\\file.txt"}\r\n`)
}

// writeCRLF writes the provided string to conn converting LF to CRLF.
// It ensures the message ends with CRLF.
func writeCRLF(conn net.Conn, s string) (int, error) {
	if s == "" {
		return conn.Write([]byte("\r\n"))
	}
	// Ensure trailing LF for consistent replacement
	if !strings.HasSuffix(s, "\n") {
		s = s + "\n"
	}
	s = strings.ReplaceAll(s, "\n", "\r\n")
	return fmt.Fprint(conn, s)
}