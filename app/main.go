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
		// This allows the main loop to immediately go back to waiting for the NEXT user.
		go handleConnection(conn, *dir)
	}
}

// handleConnection manages the lifecycle of a single TCP connection.
// It supports Persistent Connections (Keep-Alive) and Explicit Closures.
func handleConnection(conn net.Conn, dir string) {
	// Ensure the connection is closed when this function finally returns.
	defer conn.Close()

	// --- PERSISTENT CONNECTION LOOP ---
	// HTTP/1.1 connections stay open by default unless "Connection: close" is sent.
	for {
		// 1. Read Request Data
		// We allocate a 1KB buffer.
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

		request := string(buf[:n])
		
		// 2. Parse the Request Line
		lines := strings.Split(request, "\r\n")
		requestLine := strings.Split(lines[0], " ")
		
		if len(requestLine) < 2 {
			continue // Skip malformed requests
		}
		
		method := requestLine[0] // e.g., "GET", "POST"
		path := requestLine[1]   // e.g., "/", "/echo/abc"

		// --- CHECK FOR CONNECTION: CLOSE HEADER ---
		// We scan the headers to see if the client wants to close the connection after this request.
		shouldClose := false
		for _, line := range lines {
			// We check for the header. In a real server, we would use case-insensitive matching.
			if strings.HasPrefix(line, "Connection: close") {
				shouldClose = true
				break
			}
		}

		// 3. Routing Logic
		// We route the request based on the path.

		if path == "/" {
			// --- ROOT ENDPOINT ---
			// If we need to close, we explicitly add the Connection header.
			if shouldClose {
				conn.Write([]byte("HTTP/1.1 200 OK\r\nConnection: close\r\n\r\n"))
			} else {
				conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			}

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

			// ADDED: If the client asked to close, echo that back in the headers
			if shouldClose {
				headerLines = append(headerLines, "Connection: close")
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
			
			// Construct Headers manually to include Connection: close if needed
			headerLines := []string{
				"HTTP/1.1 200 OK",
				"Content-Type: text/plain",
				fmt.Sprintf("Content-Length: %d", length),
			}

			if shouldClose {
				headerLines = append(headerLines, "Connection: close")
			}

			responseHeaders := strings.Join(headerLines, "\r\n")
			response := responseHeaders + "\r\n\r\n" + userAgent
			conn.Write([]byte(response))

		} else if strings.HasPrefix(path, "/files/") {
			// --- FILE HANDLING ENDPOINT ---
			
			fileName := strings.TrimPrefix(path, "/files/")
			fullPath := filepath.Join(dir, fileName)

			if method == "POST" {
				// --- POST: CREATE FILE ---
				
				parts := strings.Split(request, "\r\n\r\n")
				if len(parts) < 2 {
					conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
					// Even on error, we respect the close header logic below
				} else {
					fileContent := parts[1]
					err := os.WriteFile(fullPath, []byte(fileContent), 0644)
					if err != nil {
						conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
					} else {
						// Success response
						if shouldClose {
							conn.Write([]byte("HTTP/1.1 201 Created\r\nConnection: close\r\n\r\n"))
						} else {
							conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))
						}
					}
				}

			} else {
				// --- GET: READ FILE ---
				
				fileData, err := os.ReadFile(fullPath)
				if err != nil {
					conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
				} else {
					length := len(fileData)
					
					headerLines := []string{
						"HTTP/1.1 200 OK",
						"Content-Type: application/octet-stream",
						fmt.Sprintf("Content-Length: %d", length),
					}
					
					if shouldClose {
						headerLines = append(headerLines, "Connection: close")
					}

					responseHeaders := strings.Join(headerLines, "\r\n")
					response := responseHeaders + "\r\n\r\n" + string(fileData)
					conn.Write([]byte(response))
				}
			}

		} else {
			// --- 404 CATCH-ALL ---
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		}

		// --- FINAL STEP: CHECK IF WE SHOULD CLOSE ---
		// If the "Connection: close" header was present, we break the loop.
		// This allows 'defer conn.Close()' to run, effectively hanging up the phone.
		if shouldClose {
			break
		}
	}
}