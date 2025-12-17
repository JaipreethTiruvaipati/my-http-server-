package main

import (
	"fmt"
	"net"
	"os"
	"strings" // Imported for string manipulation
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
	defer conn.Close() // Ensure connection closes when we are done

	// 1. Create a buffer to hold the incoming data
	// 1024 bytes is usually enough for the request headers
	buf := make([]byte, 1024)

	// 2. Read the data from the connection into the buffer
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading request:", err)
		return
	}

	// 3. Convert the read bytes to a string
	request := string(buf[:n])
	
	// 4. Parse the Request Line
	// The Request Line is the first part: "GET /index.html HTTP/1.1"
	// We split the string by spaces to get the parts
	parts := strings.Split(request, " ")
	
	// Safety check to ensure the request is valid
	if len(parts) < 2 {
		return
	}

	path := parts[1] // The path is the second element (Index 1)

	// 5. Route based on the path
	if path == "/" {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}