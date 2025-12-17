package main

import (
	"bytes"         // Needed to hold binary data in memory
	"compress/gzip" // Needed to perform the actual compression
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	// 1. Parse Command Line Flags
	// Example usage: ./server --directory /tmp/
	dir := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
		
		// Handle each connection in a new goroutine (Concurrency)
		go handleConnection(conn, *dir)
	}
}

func handleConnection(conn net.Conn, dir string) {
	defer conn.Close()

	// 1. Read the Request
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading request:", err)
		return
	}

	request := string(buf[:n])
	
	// 2. Parse Request Line
	lines := strings.Split(request, "\r\n")
	requestLine := strings.Split(lines[0], " ")
	
	if len(requestLine) < 2 {
		return
	}
	
	method := requestLine[0]
	path := requestLine[1]

	// 3. Routing Logic
	if path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	} else if strings.HasPrefix(path, "/echo/") {
		// --- ECHO with GZIP COMPRESSION Logic ---
		
		// The original string content
		content := strings.TrimPrefix(path, "/echo/")
		
		// Default: We assume no compression
		finalBody := content
		shouldCompress := false

		// A. Check headers to see if client supports 'gzip'
		for _, line := range lines {
			if strings.HasPrefix(line, "Accept-Encoding: ") {
				value := strings.TrimPrefix(line, "Accept-Encoding: ")
				if strings.Contains(value, "gzip") {
					shouldCompress = true
					break
				}
			}
		}

		// B. Perform Compression if needed
		if shouldCompress {
			// 1. Create a buffer to hold the compressed bytes
			var b bytes.Buffer
			
			// 2. Create a Gzip Writer that writes into that buffer
			w := gzip.NewWriter(&b)
			
			// 3. Write the original content into the compressor
			w.Write([]byte(content))
			
			// 4. CLOSE the writer. This is critical!
			// If you don't close, the Gzip footer won't be written, and data is corrupt.
			w.Close()
			
			// 5. Get the compressed string (Go strings can hold binary data)
			finalBody = b.String()
		}

		// C. Prepare Headers
		headerLines := []string{
			"HTTP/1.1 200 OK",
			"Content-Type: text/plain",
			// IMPORTANT: Content-Length must be the length of the COMPRESSED data
			fmt.Sprintf("Content-Length: %d", len(finalBody)),
		}

		// Add the Content-Encoding header if we actually compressed it
		if shouldCompress {
			headerLines = append(headerLines, "Content-Encoding: gzip")
		}

		// D. Send Response
		responseHeaders := strings.Join(headerLines, "\r\n")
		// \r\n\r\n separates headers from body
		response := responseHeaders + "\r\n\r\n" + finalBody
		
		conn.Write([]byte(response))

	} else if path == "/user-agent" {
		// Extract User-Agent Header
		var userAgent string
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
		// --- FILE HANDLING Logic ---
		fileName := strings.TrimPrefix(path, "/files/")
		fullPath := filepath.Join(dir, fileName)

		if method == "POST" {
			// POST: Create/Write File
			parts := strings.Split(request, "\r\n\r\n")
			if len(parts) < 2 {
				conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
				return
			}
			fileContent := parts[1]
			
			err := os.WriteFile(fullPath, []byte(fileContent), 0644)
			if err != nil {
				conn.Write([]byte("HTTP/1.1 500 Internal Server Error\r\n\r\n"))
				return
			}
			conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))

		} else {
			// GET: Read File
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