package main

import (
	"flag" // Needed to parse command line flags like --directory
	"fmt"
	"net"
	"os"
	"path/filepath" // Needed to join paths safely
	"strings"
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	// 1. Parse Command Line Flags
	// This looks for --directory and stores the value in the variable 'dir'
	// Example usage: ./server --directory /tmp/
	dir := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	// Defer ensures the listener closes when main exits (good practice)
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
		
		// CRITICAL CHANGE: The 'go' keyword makes this run in the background (Concurrency)
		// Pass the directory value (*dir) to the handler
		go handleConnection(conn, *dir)
	}
}

// handleConnection processes an individual user request
func handleConnection(conn net.Conn, dir string) {
	defer conn.Close()

	// 1. Read the Request into a buffer
	// We create a buffer of 1024 bytes to hold the incoming data
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading request:", err)
		return
	}

	// Convert the read bytes to a string for parsing
	request := string(buf[:n])
	
	// 2. Parse Request Line
	// We split by "\r\n" first to isolate the Request Line (Line 0) from Headers
	lines := strings.Split(request, "\r\n")
	requestLine := strings.Split(lines[0], " ")
	
	if len(requestLine) < 2 {
		return
	}
	
	method := requestLine[0] // e.g., "GET" or "POST"
	path := requestLine[1]   // e.g., "/files/foo" or "/index.html"

	// 3. Routing Logic
	if path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	} else if strings.HasPrefix(path, "/echo/") {
		content := strings.TrimPrefix(path, "/echo/")
		length := len(content)
		response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", length, content)
		conn.Write([]byte(response))

	} else if path == "/user-agent" {
		// Extract User-Agent Header
		var userAgent string
		// Loop through all lines to find the header starting with "User-Agent:"
		for _, line := range lines {
			if strings.HasPrefix(line, "User-Agent: ") {
				userAgent = strings.TrimPrefix(line, "User-Agent: ")
				break
			}
		}

		length := len(userAgent)
		response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", length, userAgent)
		conn.Write([]byte(response))

	} else if strings.HasPrefix(path, "/files/") {
		// Extract the filename from the URL
		fileName := strings.TrimPrefix(path, "/files/")
		// Securely combine directory + filename
		fullPath := filepath.Join(dir, fileName)

		// CHECK: Are we READING a file (GET) or CREATING a file (POST)?
		if method == "POST" {
			// --- POST LOGIC START ---
			
			// 1. Find the Body
			// The Body is separated from headers by a double CRLF ("\r\n\r\n")
			parts := strings.Split(request, "\r\n\r\n")
			
			// If we don't have a body part, just write an empty file or error out
			if len(parts) < 2 {
				conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
				return
			}
			
			// The body is the second part (Index 1)
			fileContent := parts[1]

			// Note: In a production server, we would check the 'Content-Length' header
			// to know exactly how many bytes to read, but for this stage,
			// trusting the split is usually sufficient for small files.

			// 2. Write the file to disk
			// 0644 is the permission code (Read/Write for owner, Read for others)
			err := os.WriteFile(fullPath, []byte(fileContent), 0644)
			if err != nil {
				conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
				return
			}

			// 3. Respond with 201 Created
			conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))
			
			// --- POST LOGIC END ---

		} else {
			// --- GET LOGIC (Existing) ---
			
			fileData, err := os.ReadFile(fullPath)
			if err != nil {
				conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
				return
			}

			length := len(fileData)
			response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", length, string(fileData))
			conn.Write([]byte(response))
		}

	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}