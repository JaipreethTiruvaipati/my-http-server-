package main

import (
	"fmt"
	"net"
	"os"
	"strings" // Essential for processing the path
)

func main() {
	fmt.Println("Logs from your program will appear here!")

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	conn, err := l.Accept()
	if err != nil {
		fmt.Println("Error accepting connection: ", err.Error())
		os.Exit(1)
	}
	defer conn.Close()

	// 1. Read the Request
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading request:", err)
		return
	}

	// 2. Parse the Path
	request := string(buf[:n])
	parts := strings.Split(request, " ")
	if len(parts) < 2 {
		return 
	}
	path := parts[1]

	// 3. Routing Logic
	if path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		
	} else if strings.HasPrefix(path, "/echo/") {
		// Extract the content after "/echo/"
		content := strings.TrimPrefix(path, "/echo/")
		length := len(content)

		// Construct the headers and body using Sprintf
		// %d inserts the integer length, %s inserts the string content
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

	}else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}