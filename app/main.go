package main

import (
    "fmt"
    "net"
    "os"
)

func main() {
    fmt.Println("Logs from your program will appear here!")

    l, err := net.Listen("tcp", "0.0.0.0:4221")
    if err != nil {
        fmt.Println("Failed to bind to port 4221")
        os.Exit(1)
    }
    
    // Capture the connection object ('conn') instead of discarding it with '_'
    conn, err := l.Accept()
    if err != nil {
        fmt.Println("Error accepting connection: ", err.Error())
        os.Exit(1)
    }

    // Write the HTTP response to the connection
    // We convert the string to a byte slice because TCP transmits raw bytes
    conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
    
    // Close the connection to signal we are done sending data
    conn.Close()
}