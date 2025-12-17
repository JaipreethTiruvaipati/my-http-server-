package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// --- 1. CORE TYPES ---

// HTTPRequest helps us pass parsed data around easily (Decoupling)
type HTTPRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    string
}

// HandlerFunc defines the shape of function our Router can call.
// It takes the connection, the parsed request, and the config (dir).
type HandlerFunc func(conn net.Conn, req HTTPRequest, dir string)

// Route represents a single path mapping
type Route struct {
	Method  string
	Path    string
	Handler HandlerFunc
	IsPrefix bool // If true, matches "/path/..." instead of exact "/path"
}

// Router manages the list of routes and finds the right handler
type Router struct {
	routes []Route
}

// NewRouter creates a ready-to-use router
func NewRouter() *Router {
	return &Router{routes: []Route{}}
}

// Register adds a new route to the list
func (r *Router) Register(method, path string, handler HandlerFunc) {
	// Simple logic: if path ends in slash, treat as prefix match (e.g., /echo/)
	isPrefix := strings.HasSuffix(path, "/")
	r.routes = append(r.routes, Route{
		Method:   method,
		Path:     path,
		Handler:  handler,
		IsPrefix: isPrefix,
	})
}

// Helper methods for cleaner registration
func (r *Router) Get(path string, handler HandlerFunc) {
	r.Register("GET", path, handler)
}
func (r *Router) Post(path string, handler HandlerFunc) {
	r.Register("POST", path, handler)
}

// ServeRequest is the "Receptionist": checks the path and calls the right Handler
func (r *Router) ServeRequest(conn net.Conn, req HTTPRequest, dir string) {
	for _, route := range r.routes {
		matches := false
		
		if route.Method != req.Method {
			continue
		}

		if route.IsPrefix {
			if strings.HasPrefix(req.Path, route.Path) {
				matches = true
			}
		} else {
			if req.Path == route.Path {
				matches = true
			}
		}

		if matches {
			route.Handler(conn, req, dir)
			return
		}
	}

	// 404 Catch-all
	sendResponse(conn, "404 Not Found", nil, "", req)
}

// --- 2. HANDLERS (The "Specialists") ---

func rootHandler(conn net.Conn, req HTTPRequest, dir string) {
	sendResponse(conn, "200 OK", nil, "", req)
}

func echoHandler(conn net.Conn, req HTTPRequest, dir string) {
	content := strings.TrimPrefix(req.Path, "/echo/")
	
	// Check for GZIP support
	encoding := req.Headers["Accept-Encoding"]
	shouldCompress := strings.Contains(encoding, "gzip")
	
	finalBody := content
	extraHeaders := make(map[string]string)
	extraHeaders["Content-Type"] = "text/plain"

	if shouldCompress {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		w.Write([]byte(content))
		w.Close()
		finalBody = b.String()
		extraHeaders["Content-Encoding"] = "gzip"
	}

	sendResponse(conn, "200 OK", extraHeaders, finalBody, req)
}

func userAgentHandler(conn net.Conn, req HTTPRequest, dir string) {
	agent := req.Headers["User-Agent"]
	headers := map[string]string{"Content-Type": "text/plain"}
	sendResponse(conn, "200 OK", headers, agent, req)
}

func getFileHandler(conn net.Conn, req HTTPRequest, dir string) {
	fileName := strings.TrimPrefix(req.Path, "/files/")
	fullPath := filepath.Join(dir, fileName)

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		sendResponse(conn, "404 Not Found", nil, "", req)
		return
	}

	headers := map[string]string{"Content-Type": "application/octet-stream"}
	sendResponse(conn, "200 OK", headers, string(fileData), req)
}

func createFileHandler(conn net.Conn, req HTTPRequest, dir string) {
	fileName := strings.TrimPrefix(req.Path, "/files/")
	fullPath := filepath.Join(dir, fileName)

	err := os.WriteFile(fullPath, []byte(req.Body), 0644)
	if err != nil {
		sendResponse(conn, "500 Internal Server Error", nil, "", req)
		return
	}
	sendResponse(conn, "201 Created", nil, "", req)
}

// --- 3. MAIN SERVER LOGIC ---

func main() {
	dir := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	// Initialize Router
	router := NewRouter()
	
	// Register Routes (Dynamic Registration)
	router.Get("/", rootHandler)
	router.Get("/echo/", echoHandler)       // Ends with / -> Prefix match
	router.Get("/user-agent", userAgentHandler)
	router.Get("/files/", getFileHandler)
	router.Post("/files/", createFileHandler)

	fmt.Println("Server running on port 4221...")
	
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn, router, *dir)
	}
}

// handleConnection: Reads raw bytes, parses them, and delegates to Router
func handleConnection(conn net.Conn, router *Router, dir string) {
	defer conn.Close()

	for {
		// 1. Read Request
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err == io.EOF || n == 0 { break }
		if err != nil { break }

		rawRequest := string(buf[:n])
		
		// 2. Parse Request
		req, isValid := parseRequest(rawRequest)
		if !isValid { continue }

		// 3. Delegate to Router
		router.ServeRequest(conn, req, dir)

		// 4. Handle "Connection: close"
		if val, ok := req.Headers["Connection"]; ok && val == "close" {
			break
		}
	}
}

// --- 4. HELPERS ---

func parseRequest(raw string) (HTTPRequest, bool) {
	parts := strings.Split(raw, "\r\n\r\n")
	headerPart := parts[0]
	body := ""
	if len(parts) > 1 {
		body = parts[1]
	}

	lines := strings.Split(headerPart, "\r\n")
	requestLine := strings.Split(lines[0], " ")
	if len(requestLine) < 2 {
		return HTTPRequest{}, false
	}

	req := HTTPRequest{
		Method:  requestLine[0],
		Path:    requestLine[1],
		Headers: make(map[string]string),
		Body:    body,
	}

	for _, line := range lines[1:] {
		if colonIdx := strings.Index(line, ": "); colonIdx != -1 {
			key := line[:colonIdx]
			val := line[colonIdx+2:]
			req.Headers[key] = val
		}
	}
	return req, true
}

func sendResponse(conn net.Conn, status string, headers map[string]string, body string, req HTTPRequest) {
	response := []string{fmt.Sprintf("HTTP/1.1 %s", status)}
	
	for k, v := range headers {
		response = append(response, fmt.Sprintf("%s: %s", k, v))
	}
	
	response = append(response, fmt.Sprintf("Content-Length: %d", len(body)))
	
	// Echo connection header if strictly needed (Keep-Alive logic)
	if val, ok := req.Headers["Connection"]; ok && val == "close" {
		response = append(response, "Connection: close")
	}
	
	finalResp := strings.Join(response, "\r\n") + "\r\n\r\n" + body
	conn.Write([]byte(finalResp))
}