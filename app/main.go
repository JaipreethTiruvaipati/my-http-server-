package main

import (
	"bytes"         // Used to manipulate byte buffers (for compression)
	"compress/gzip" // Used to compress data using the GZIP algorithm
	"flag"          // Used to parse command-line arguments (flags)
	"fmt"           // Used for formatted I/O (printing to console)
	"io"            // Used to handle input/output errors like EOF
	"net"           // Used for network I/O (TCP sockets)
	"os"            // Used for operating system functionality (File I/O, Exit)
	"path/filepath" // Used to construct file paths safely across OSs
	"strings"       // Used for string manipulation (splitting, prefix checks)
)

func main() {
	// 1. Parse Command Line Flags
	// The user can start the server with: ./server --directory /tmp/
	// If the flag isn't provided, it defaults to "." (current directory).
	dir := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	fmt.Println("Logs from your program will appear here!")

	// 2. Create the TCP Listener
	// We bind to 0.0.0.0 (all interfaces) on port 4221.
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	// 'defer' ensures the listener is closed if the main function exits unexpectedly.
	defer l.Close()

	// 3. The Main Connection Loop
	// This loop runs forever, waiting for new users to connect.
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
		
		// 4. Concurrency (Goroutines)
		// The 'go' keyword spawns a lightweight thread.
		// This allows the main loop to immediately go back to waiting for the NEXT user,
		// while 'handleConnection' deals with the CURRENT user in the background.
		go handleConnection(conn, *dir)
	}
}

// handleConnection manages the lifecycle of a single TCP connection.
// It supports Persistent Connections (Keep-Alive) by looping until the client disconnects.
func handleConnection(conn net.Conn, dir string) {
	// Ensure the connection is closed when this function finally returns.
	defer conn.Close()

	// --- PERSISTENT CONNECTION LOOP ---
	// HTTP/1.1 connections stay open by default to handle multiple requests sequentially.
	for {
		// 1. Read Request Data
		// We allocate a 1KB buffer. For a production server, we would need dynamic buffering.
		buf := make([]byte, 1024)
		
		n, err := conn.Read(buf)
		
		// Handle Disconnection:
		// io.EOF means the client (browser/curl) has closed the connection cleanly.
		if err == io.EOF {
			break // Exit the loop to close the connection
		}
		if err != nil {
			fmt.Println("Error reading request:", err)
			break
		}
		// If 0 bytes were read, the connection is effectively dead.
		if n == 0 {
			break
		}

		// Convert the buffer to a string for parsing.
		// Note: We only convert buf[:n] to avoid reading empty garbage data at the end.
		request := string(buf[:n])
		
		// 2. Parse the Request Line
		// HTTP requests use CRLF (\r\n) to separate lines.
		lines := strings.Split(request, "\r\n")
		
		// The first line (Request Line) looks like: "GET /index.html HTTP/1.1"
		requestLine := strings.Split(lines[0], " ")
		
		if len(requestLine) < 2 {
			continue // Skip malformed requests
		}
		
		method := requestLine[0] // e.g., "GET", "POST"
		path := requestLine[1]   // e.g., "/", "/echo/abc"

		// 3. Routing Logic
		// We route the request based on the path.

		if path == "/" {
			// --- ROOT ENDPOINT ---
			conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

		} else if strings.HasPrefix(path, "/echo/") {
			// --- ECHO ENDPOINT (With GZIP) ---
			
			content := strings.TrimPrefix(path, "/echo/")
			
			// Compression Logic
			finalBody := content
			shouldCompress := false
			
			// Check headers for 'Accept-Encoding: gzip'
			for _, line := range lines {
				if strings.HasPrefix(line, "Accept-Encoding: ") {
					value := strings.TrimPrefix(line, "Accept-Encoding: ")
					if strings.Contains(value, "gzip") {
						shouldCompress = true
						break
					}
				}
			}

			// If client supports gzip, compress the body
			if shouldCompress {
				var b bytes.Buffer
				w := gzip.NewWriter(&b)
				w.Write([]byte(content))
				w.Close() // Must close to write the Gzip footer/checksum
				finalBody = b.String()
			}

			// Construct Headers
			headerLines := []string{
				"HTTP/1.1 200 OK",
				"Content-Type: text/plain",
				// Content-Length matches the size of the body (compressed or not)
				fmt.Sprintf("Content-Length: %d", len(finalBody)),
			}

			if shouldCompress {
				headerLines = append(headerLines, "Content-Encoding: gzip")
			}

			// Join headers and body with CRLFs
			responseHeaders := strings.Join(headerLines, "\r\n")
			response := responseHeaders + "\r\n\r\n" + finalBody
			
			conn.Write([]byte(response))

		} else if path == "/user-agent" {
			// --- USER-AGENT ENDPOINT ---
			
			var userAgent string
			// Scan headers to find User-Agent
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
			// --- FILE HANDLING ENDPOINT ---
			
			fileName := strings.TrimPrefix(path, "/files/")
			// filepath.Join handles OS-specific separators (Slash vs Backslash)
			fullPath := filepath.Join(dir, fileName)

			if method == "POST" {
				// --- POST: CREATE FILE ---
				
				// Extract Body: split request by the double CRLF delimiter
				parts := strings.Split(request, "\r\n\r\n")
				if len(parts) < 2 {
					conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
					continue
				}
				fileContent := parts[1]
				
				// Write data to disk (0644 = Read/Write permission)
				err := os.WriteFile(fullPath, []byte(fileContent), 0644)
				if err != nil {
					conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
					continue
				}
				conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))

			} else {
				// --- GET: READ FILE ---
				
				fileData, err := os.ReadFile(fullPath)
				if err != nil {
					conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
					continue
				}
				length := len(fileData)
				response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s", length, string(fileData))
				conn.Write([]byte(response))
			}

		} else {
			// --- 404 CATCH-ALL ---
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		}
	}
}