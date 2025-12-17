package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
        
        // Handle connection in a separate function or block to keep main clean
        // For now, we keep it inline for simplicity
		handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// 1. Read Request
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading request:", err)
		return
	}

	request := string(buf[:n])
	
	// 2. Parse Request Line (to get the Path)
	// We split by "\r\n" first to isolate the Request Line (Line 0) from Headers
	lines := strings.Split(request, "\r\n")
	requestLine := strings.Split(lines[0], " ")
	
	if len(requestLine) < 2 {
		return
	}
	
	path := requestLine[1]

	// 3. Routing Logic
	if path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	} else if strings.HasPrefix(path, "/echo/") {
		content := strings.TrimPrefix(path, "/echo/")
		length := len(content)
		response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", length, content)
		conn.Write([]byte(response))

	} else if path == "/user-agent" {
		// 4. Extract User-Agent Header
		var userAgent string
		
		// Loop through all lines to find the header starting with "User-Agent:"
		for _, line := range lines {
			if strings.HasPrefix(line, "User-Agent: ") {
				// Cut off the "User-Agent: " part to get the value
				userAgent = strings.TrimPrefix(line, "User-Agent: ")
				break
			}
		}

		length := len(userAgent)
		response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", length, userAgent)
		conn.Write([]byte(response))

	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}