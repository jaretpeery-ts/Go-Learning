package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	enumWindowsProc          = user32.NewProc("EnumWindows")
	getWindowTextWProc       = user32.NewProc("GetWindowTextW")
	setForegroundWindowProc  = user32.NewProc("SetForegroundWindow")
	getForegroundWindowProc  = user32.NewProc("GetForegroundWindow")
	showWindowProc           = user32.NewProc("ShowWindow")
	isWindowVisibleProc      = user32.NewProc("IsWindowVisible")
	procKeybd_event          = user32.NewProc("keybd_event")
	logFile                  *os.File
	logFilePath              string = "TCP-Keyboard-server.log" // Default log file path
	serverListener           net.Listener
)

func init() {
	// Parse log file path from command line
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-l" && i+1 < len(os.Args) {
			logFilePath = os.Args[i+1]
			i++ // Skip the next argument since we used it
		}
	}

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	defer logFile.Close()

	// Check for help command line parameter
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "-h" || os.Args[1] == "--help") {
		printStartupInfo()
		return
	}

	log.Println("Running as console application (recommended to run under NSSM)")
	runServer()
}

func runServer() {
	var err error
	serverListener, err = net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	defer serverListener.Close()

	log.Println("\n=== SERVER STARTED ===")
	log.Println("Server listening on :9000")
	log.Printf("Log file: %s\n", logFilePath)
	log.Println("Waiting for connections...")
	log.Println("(Run with 'help' parameter for command reference)")

	// Handle graceful shutdown (CTRL+C when run interactively)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Shutdown signal received (%v), closing server...\n", sig)
		_ = serverListener.Close()
		os.Exit(0)
	}()

	// Accept incoming connections
	for {
		conn, err := serverListener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			break
		}

		// Handle connection in a goroutine
		go handleConnection(conn)
	}
}


func printStartupInfo() {
	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║        Redline TCP Keyboard Server - Command Reference        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")

	fmt.Println("\n📋 COMMAND LINE PARAMETERS:")
	fmt.Println("\n  help, -h, --help: Display this help information")
	fmt.Println("  -l <log_file_path>: Specify custom log file path (default: TCP-Keyboard-server.log)")
	fmt.Println("\nExample: .\\TCP-Keyboard.exe -l C:\\logs\\keyboard.log")

	fmt.Println("\n📋 ALLOWED TCP MESSAGE STRUCTURES:")

	fmt.Println("\n1. List Visible Windows (recommended):")
	fmt.Println("   {\"action\":\"list_visible_windows\"}")
	fmt.Println("   Response: {\"status\":\"success\",\"windows\":[\"Window1\",\"Window2\",...]}")
	fmt.Println("   - Shows only visible windows you can see on screen")

	fmt.Println("\n2. List All Windows:")
	fmt.Println("   {\"action\":\"list_all_windows\"}")
	fmt.Println("   Response: {\"status\":\"success\",\"windows\":[\"Window1\",\"Window2\",...]}")
	fmt.Println("   - Shows all windows including hidden/background processes")

	fmt.Println("\n3. Press Keys:")
	fmt.Println("   {\"action\":\"keypress\",\"window_title\":\"Window Title\",\"keys\":[\"a\",\"b\",\"c\"]}")
	fmt.Println("   - window_title: Partial match of window title (case-insensitive)")
	fmt.Println("   - keys: Array of key names to press sequentially")
	fmt.Println("   Response: {\"status\":\"success\",\"message\":\"Pressed keys...in window...\"}")

	fmt.Println("\n⌨️  ACCEPTED KEYS:")
	allowedKeys := getAllowedKeys()

	fmt.Println("\n  Alphabet (a-z):")
	fmt.Print("    ")
	for _, k := range allowedKeys["alphabet"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Numbers (0-9):")
	fmt.Print("    ")
	for _, k := range allowedKeys["numbers"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Function Keys:")
	fmt.Print("    ")
	for _, k := range allowedKeys["function"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Control Keys:")
	fmt.Print("    ")
	for _, k := range allowedKeys["control"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Arrow Keys:")
	fmt.Print("    ")
	for _, k := range allowedKeys["arrows"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Modifier Keys (held until end):")
	fmt.Print("    ")
	for _, k := range allowedKeys["modifiers"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Special Characters:")
	fmt.Print("    ")
	for _, k := range allowedKeys["special"] {
		fmt.Print(k + " ")
	}
	fmt.Println()

	fmt.Println("\n  Numpad Keys:")
	fmt.Print("    ")
	for _, k := range allowedKeys["numpad"] {
		fmt.Print(k + " ")
	}
	fmt.Println("")
}

func getAllowedKeys() map[string][]string {
	return map[string][]string{
		"alphabet":  {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"},
		"numbers":   {"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"},
		"function":  {"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"},
		"control":   {"enter", "return", "tab", "backspace", "space", "escape", "delete", "insert", "home", "end", "pageup", "pagedown"},
		"arrows":    {"left", "up", "right", "down"},
		"modifiers": {"shift", "ctrl", "control", "alt", "capslock", "caps", "numlock", "scroll", "menu", "super", "win"},
		"special":   {"!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "-", "_", "=", "+", "[", "{", "]", "}", ";", ":", "'", "\"", ",", "<", ".", ">", "/", "?", "`", "~"},
		"numpad":    {"numpad0", "numpad1", "numpad2", "numpad3", "numpad4", "numpad5", "numpad6", "numpad7", "numpad8", "numpad9", "numpad*", "numpad+", "numpad-", "numpad.", "numpad/"},
	}
}

func isModifierKey(key string) bool {
	switch key {
	case "shift", "ctrl", "control", "alt", "capslock", "caps", "numlock", "scroll", "menu", "super", "win":
		return true
	}
	return false
}

func waitForForeground(hwnd syscall.Handle, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		fg, _, _ := getForegroundWindowProc.Call()
		if syscall.Handle(fg) == hwnd {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

type ListWindowsRequest struct {
	Action string `json:"action"`
}

type KeypressRequest struct {
	Action      string   `json:"action"`
	WindowTitle string   `json:"window_title"`
	Keys        []string `json:"keys"`
}

func parseMessage(message string) string {
	if len(message) == 0 {
		return toJSON("error", "Empty message", nil)
	}

	// First, decode to get the action type
	var actionOnly struct {
		Action string `json:"action"`
	}
	err := json.Unmarshal([]byte(message), &actionOnly)
	if err != nil {
		return toJSON("error", "Invalid JSON: "+err.Error(), nil)
	}

	// Route based on action
	switch actionOnly.Action {
	case "list_visible_windows":
		var req ListWindowsRequest
		if err := json.Unmarshal([]byte(message), &req); err != nil {
			return toJSON("error", "Invalid list_visible_windows request: "+err.Error(), nil)
		}
		return handleListVisibleWindows()

	case "list_all_windows":
		var req ListWindowsRequest
		if err := json.Unmarshal([]byte(message), &req); err != nil {
			return toJSON("error", "Invalid list_all_windows request: "+err.Error(), nil)
		}
		return handleListAllWindows()

	case "keypress":
		var req KeypressRequest
		if err := json.Unmarshal([]byte(message), &req); err != nil {
			return toJSON("error", "Invalid keypress request: "+err.Error(), nil)
		}
		// Validate required fields
		if req.WindowTitle == "" {
			return toJSON("error", "Missing window_title field", nil)
		}
		if len(req.Keys) == 0 {
			return toJSON("error", "Missing or empty keys array", nil)
		}
		return handleKeypress(req.WindowTitle, req.Keys)

	default:
		return toJSON("error", "Unknown action: "+actionOnly.Action, nil)
	}
}

func handleListVisibleWindows() string {
	var windowTitles []string
	enumWindowsProc.Call(syscall.NewCallback(func(h syscall.Handle, lparam uintptr) uintptr {
		// Only include visible windows
		visible, _, _ := isWindowVisibleProc.Call(uintptr(h))
		if visible == 0 {
			return 1 // Skip hidden windows
		}

		var title [256]uint16
		getWindowTextWProc.Call(uintptr(h), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		windowTitle := syscall.UTF16ToString(title[:])
		// Only include windows with titles
		if windowTitle != "" {
			windowTitles = append(windowTitles, windowTitle)
		}
		return 1 // Continue enumeration
	}), 0)

	response := map[string]interface{}{
		"status":  "success",
		"windows": windowTitles,
	}
	jsonResp, _ := json.Marshal(response)
	return string(jsonResp)
}

func handleListAllWindows() string {
	var windowTitles []string
	enumWindowsProc.Call(syscall.NewCallback(func(h syscall.Handle, lparam uintptr) uintptr {
		var title [256]uint16
		getWindowTextWProc.Call(uintptr(h), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		windowTitle := syscall.UTF16ToString(title[:])
		// Include all windows with titles (both visible and hidden)
		if windowTitle != "" {
			windowTitles = append(windowTitles, windowTitle)
		}
		return 1 // Continue enumeration
	}), 0)

	response := map[string]interface{}{
		"status":  "success",
		"windows": windowTitles,
	}
	jsonResp, _ := json.Marshal(response)
	return string(jsonResp)
}

func handleKeypress(windowTitle string, keys []string) string {
	var hwnd syscall.Handle
	var allWindows []string

	enumWindowsProc.Call(syscall.NewCallback(func(h syscall.Handle, lparam uintptr) uintptr {
		var title [256]uint16
		getWindowTextWProc.Call(uintptr(h), uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		windowTitleStr := syscall.UTF16ToString(title[:])

		if windowTitleStr != "" {
			allWindows = append(allWindows, windowTitleStr)
		}
		if strings.Contains(strings.ToLower(windowTitleStr), strings.ToLower(windowTitle)) {
			hwnd = h
			return 0 // Stop enumeration on first match
		}
		return 1 // Continue enumeration
	}), 0)

	if hwnd == 0 {
		errorData := map[string]interface{}{
			"searched_for":      windowTitle,
			"available_windows": allWindows,
		}
		log.Printf("Window not found: '%s'. Available windows: %v\n", windowTitle, allWindows)
		return toJSON("error", fmt.Sprintf("Window not found: '%s'. Use 'list_windows' action to see available windows.", windowTitle), errorData)
	}

	// If the target window is minimized, restore it first so it can receive focus
	const SW_RESTORE = 9
	showWindowProc.Call(uintptr(hwnd), uintptr(SW_RESTORE))

	// Try a little harder to get focus reliably
	for i := 0; i < 5; i++ {
		setForegroundWindowProc.Call(uintptr(hwnd))
		if waitForForeground(hwnd, 350*time.Millisecond) {
			break
		}
		time.Sleep(75 * time.Millisecond)
	}

	if !waitForForeground(hwnd, 100*time.Millisecond) {
		// Not fatal, but very useful to log
		log.Printf("Warning: target window did not become foreground: '%s'\n", windowTitle)
		return toJSON("error", fmt.Sprintf("Failed to focus window: '%s'", windowTitle), nil)
	}

	var pressedKeys []string
	var heldModifiers []byte // Track held modifier keys

	for _, key := range keys {
		var vkCode byte
		switch strings.ToLower(key) {
		// Alphabet
		case "a":
			vkCode = 0x41
		case "b":
			vkCode = 0x42
		case "c":
			vkCode = 0x43
		case "d":
			vkCode = 0x44
		case "e":
			vkCode = 0x45
		case "f":
			vkCode = 0x46
		case "g":
			vkCode = 0x47
		case "h":
			vkCode = 0x48
		case "i":
			vkCode = 0x49
		case "j":
			vkCode = 0x4A
		case "k":
			vkCode = 0x4B
		case "l":
			vkCode = 0x4C
		case "m":
			vkCode = 0x4D
		case "n":
			vkCode = 0x4E
		case "o":
			vkCode = 0x4F
		case "p":
			vkCode = 0x50
		case "q":
			vkCode = 0x51
		case "r":
			vkCode = 0x52
		case "s":
			vkCode = 0x53
		case "t":
			vkCode = 0x54
		case "u":
			vkCode = 0x55
		case "v":
			vkCode = 0x56
		case "w":
			vkCode = 0x57
		case "x":
			vkCode = 0x58
		case "y":
			vkCode = 0x59
		case "z":
			vkCode = 0x5A

		// Numbers
		case "0":
			vkCode = 0x30
		case "1":
			vkCode = 0x31
		case "2":
			vkCode = 0x32
		case "3":
			vkCode = 0x33
		case "4":
			vkCode = 0x34
		case "5":
			vkCode = 0x35
		case "6":
			vkCode = 0x36
		case "7":
			vkCode = 0x37
		case "8":
			vkCode = 0x38
		case "9":
			vkCode = 0x39

		// Function keys
		case "f1":
			vkCode = 0x70
		case "f2":
			vkCode = 0x71
		case "f3":
			vkCode = 0x72
		case "f4":
			vkCode = 0x73
		case "f5":
			vkCode = 0x74
		case "f6":
			vkCode = 0x75
		case "f7":
			vkCode = 0x76
		case "f8":
			vkCode = 0x77
		case "f9":
			vkCode = 0x78
		case "f10":
			vkCode = 0x79
		case "f11":
			vkCode = 0x7A
		case "f12":
			vkCode = 0x7B

		// Control keys
		case "enter", "return":
			vkCode = 0x0D
		case "tab":
			vkCode = 0x09
		case "backspace":
			vkCode = 0x08
		case "space":
			vkCode = 0x20
		case "escape":
			vkCode = 0x1B
		case "delete":
			vkCode = 0x2E
		case "insert":
			vkCode = 0x2D
		case "home":
			vkCode = 0x24
		case "end":
			vkCode = 0x23
		case "pageup":
			vkCode = 0x21
		case "pagedown":
			vkCode = 0x22

		// Arrow keys
		case "left":
			vkCode = 0x25
		case "up":
			vkCode = 0x26
		case "right":
			vkCode = 0x27
		case "down":
			vkCode = 0x28

		// Special characters
		case "!":
			vkCode = 0x31 // Shift+1
		case "@":
			vkCode = 0x32 // Shift+2
		case "#":
			vkCode = 0x33 // Shift+3
		case "$":
			vkCode = 0x34 // Shift+4
		case "%":
			vkCode = 0x35 // Shift+5
		case "^":
			vkCode = 0x36 // Shift+6
		case "&":
			vkCode = 0x37 // Shift+7
		case "*":
			vkCode = 0x38 // Shift+8
		case "(":
			vkCode = 0x39 // Shift+9
		case ")":
			vkCode = 0x30 // Shift+0
		case "-":
			vkCode = 0xBD
		case "_":
			vkCode = 0xBD // Shift+-
		case "=":
			vkCode = 0xBB
		case "+":
			vkCode = 0xBB // Shift+=
		case "[":
			vkCode = 0xDB
		case "{":
			vkCode = 0xDB // Shift+[
		case "]":
			vkCode = 0xDD
		case "}":
			vkCode = 0xDD // Shift+]
		case ";":
			vkCode = 0xBA
		case ":":
			vkCode = 0xBA // Shift+;
		case "'":
			vkCode = 0xDE
		case "\"":
			vkCode = 0xDE // Shift+'
		case ",":
			vkCode = 0xBC
		case "<":
			vkCode = 0xBC // Shift+,
		case ".":
			vkCode = 0xBE
		case ">":
			vkCode = 0xBE // Shift+.
		case "/":
			vkCode = 0xBF
		case "?":
			vkCode = 0xBF // Shift+/
		case "`":
			vkCode = 0xC0
		case "~":
			vkCode = 0xC0 // Shift+`

		// Modifier keys
		case "shift":
			vkCode = 0x10
		case "ctrl", "control":
			vkCode = 0x11
		case "alt":
			vkCode = 0x12
		case "capslock", "caps":
			vkCode = 0x14
		case "numlock":
			vkCode = 0x90
		case "scroll":
			vkCode = 0x91
		case "printscreen":
			vkCode = 0x2C
		case "pause":
			vkCode = 0x13
		case "menu":
			vkCode = 0x5D
		case "super", "win":
			vkCode = 0x5B

		// Numpad
		case "numpad0":
			vkCode = 0x60
		case "numpad1":
			vkCode = 0x61
		case "numpad2":
			vkCode = 0x62
		case "numpad3":
			vkCode = 0x63
		case "numpad4":
			vkCode = 0x64
		case "numpad5":
			vkCode = 0x65
		case "numpad6":
			vkCode = 0x66
		case "numpad7":
			vkCode = 0x67
		case "numpad8":
			vkCode = 0x68
		case "numpad9":
			vkCode = 0x69
		case "numpad*":
			vkCode = 0x6A
		case "numpad+":
			vkCode = 0x6B
		case "numpad-":
			vkCode = 0x6D
		case "numpad.":
			vkCode = 0x6E
		case "numpad/":
			vkCode = 0x6F

		default:
			return toJSON("error", "Unknown key: "+key, nil)
		}

		// Check if this is a modifier key
		if isModifierKey(strings.ToLower(key)) {
			// Press modifier and add to held list
			procKeybd_event.Call(uintptr(vkCode), 0, 0, 0)
			heldModifiers = append(heldModifiers, vkCode)
			time.Sleep(50 * time.Millisecond)
		} else {
			// Regular key: press and release
			procKeybd_event.Call(uintptr(vkCode), 0, 0, 0)
			time.Sleep(50 * time.Millisecond)
			procKeybd_event.Call(uintptr(vkCode), 0, 0x0002, 0)
			time.Sleep(50 * time.Millisecond)
		}

		pressedKeys = append(pressedKeys, key)
	}

	// Release all held modifier keys at the end
	for _, vkCode := range heldModifiers {
		procKeybd_event.Call(uintptr(vkCode), 0, 0x0002, 0)
		time.Sleep(50 * time.Millisecond)
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Pressed keys %v in window '%s'", pressedKeys, windowTitle),
	}
	jsonResp, _ := json.Marshal(response)
	return string(jsonResp)
}

func toJSON(status string, message string, data interface{}) string {
	response := map[string]interface{}{
		"status":  status,
		"message": message,
	}
	if data != nil {
		response["data"] = data
	}
	jsonResp, _ := json.Marshal(response)
	return string(jsonResp)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("Client connected: %s\n", conn.RemoteAddr())

	// Use Scanner to handle line delimiters dynamically
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "" {
			continue
		}

		parsedMessage := parseMessage(message)
		log.Printf("Received from %s: %s\n", conn.RemoteAddr(), parsedMessage)

		// Send response back to client
		_, err := conn.Write([]byte(parsedMessage + "\n"))
		if err != nil {
			log.Println("Write error:", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Client error: %s\n", err)
	}
	log.Printf("Client disconnected: %s\n", conn.RemoteAddr())
}