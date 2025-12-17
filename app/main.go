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

// =================================================================================
// PART 1: DEFINING THE DESIGN PATTERN (Router & Handler)
//
// Concept: Decoupling.
// - The "Router" acts like a receptionist. It knows WHO handles a request but
//   doesn't know HOW to do the job.
// - The "Handler" acts like a specialist (Chef, Maid). It knows HOW to do the
//   job but doesn't care about how the customer arrived there.
// =================================================================================

// HTTPRequest struct:
// Instead of passing raw strings around, we parse the request into a struct.
// This makes the code cleaner and type-safe.
type HTTPRequest struct {
	Method  string            // e.g., "GET", "POST"
	Path    string            // e.g., "/echo/hello"
	Headers map[string]string // Key-Value pairs, e.g., "User-Agent" -> "curl/7.64.1"
	Body    string            // The data sent after the headers (if any)
}

// HandlerFunc Type Definition:
// This is a "Function Type". It defines the signature that ALL handlers must follow.
// By enforcing this standard, the Router can treat all handlers (echo, files, root) exactly the same.
type HandlerFunc func(conn net.Conn, req HTTPRequest, dir string)

// Route struct:
// Represents a single entry in our routing table.
type Route struct {
	Method   string      // The HTTP method required (GET/POST)
	Path     string      // The URL path to match
	Handler  HandlerFunc // The function to execute if matched
	IsPrefix bool        // If true, matches "/path/..." (useful for dynamic paths like /echo/abc)
}

// Router struct:
// The manager that holds all the routes.
type Router struct {
	routes []Route
}

// NewRouter initializes an empty router.
func NewRouter() *Router {
	return &Router{routes: []Route{}}
}

// Register adds a new route to the router.
// This allows us to add paths DYNAMICALLY without changing the main loop code.
func (r *Router) Register(method, path string, handler HandlerFunc) {
	// Logic: If the path ends in "/", we treat it as a "prefix match" (e.g., /echo/anything).
	// Otherwise, it's an "exact match" (e.g., /user-agent).
	isPrefix := strings.HasSuffix(path, "/")
	
	r.routes = append(r.routes, Route{
		Method:   method,
		Path:     path,
		Handler:  handler,
		IsPrefix: isPrefix,
	})
}

// Helper methods (Get/Post) make the registration code look cleaner (Syntactic Sugar).
func (r *Router) Get(path string, handler HandlerFunc) {
	r.Register("GET", path, handler)
}
func (r *Router) Post(path string, handler HandlerFunc) {
	r.Register("POST", path, handler)
}

// ServeRequest is the core logic of the Router.
// It iterates through the registered routes to find a match for the incoming request.
func (r *Router) ServeRequest(conn net.Conn, req HTTPRequest, dir string) {
	for _, route := range r.routes {
		// 1. Check if the HTTP Method matches (GET vs POST)
		if route.Method != req.Method {
			continue
		}

		matches := false
		// 2. Check if the Path matches
		if route.IsPrefix {
			// Prefix Match: e.g., Request "/echo/abc" matches Route "/echo/"
			if strings.HasPrefix(req.Path, route.Path) {
				matches = true
			}
		} else {
			// Exact Match: e.g., Request "/user-agent" matches Route "/user-agent"
			if req.Path == route.Path {
				matches = true
			}
		}

		// 3. If matched, execute the specific handler and return immediately.
		if matches {
			route.Handler(conn, req, dir)
			return
		}
	}

	// 4. Fallback: If no route matched, send a 404.
	sendResponse(conn, "404 Not Found", nil, "", req)
}

// =================================================================================
// PART 2: THE HANDLERS (The Business Logic)
// Each function below is isolated. It doesn't know about TCP loops or parsing.
// =================================================================================

// rootHandler handles requests to "/"
func rootHandler(conn net.Conn, req HTTPRequest, dir string) {
	sendResponse(conn, "200 OK", nil, "", req)
}

// echoHandler handles "/echo/{str}"
// Demonstrates: String manipulation and conditional logic (Gzip).
func echoHandler(conn net.Conn, req HTTPRequest, dir string) {
	// Extract the content by removing the prefix "/echo/"
	content := strings.TrimPrefix(req.Path, "/echo/")
	
	// Check if the client accepts Gzip compression
	encoding := req.Headers["Accept-Encoding"]
	shouldCompress := strings.Contains(encoding, "gzip")
	
	finalBody := content
	extraHeaders := make(map[string]string)
	extraHeaders["Content-Type"] = "text/plain"

	// Application Logic: Compress data if requested
	if shouldCompress {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		w.Write([]byte(content))
		w.Close() // Important: Close writes the Gzip checksum/footer
		finalBody = b.String()
		extraHeaders["Content-Encoding"] = "gzip"
	}

	sendResponse(conn, "200 OK", extraHeaders, finalBody, req)
}

// userAgentHandler handles "/user-agent"
// Demonstrates: Reading headers from the request struct.
func userAgentHandler(conn net.Conn, req HTTPRequest, dir string) {
	agent := req.Headers["User-Agent"]
	headers := map[string]string{"Content-Type": "text/plain"}
	sendResponse(conn, "200 OK", headers, agent, req)
}

// optionsHandler handles CORS preflight "OPTIONS" requests for any path.
// This allows the browser (running the frontend on a different port) to
// verify that cross-origin requests are permitted.
func optionsHandler(conn net.Conn, req HTTPRequest, dir string) {
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Accept, Accept-Encoding, X-Requested-With",
	}
	sendResponse(conn, "204 No Content", headers, "", req)
}

// getFileHandler handles GET "/files/{filename}"
// Demonstrates: Safe file reading using path/filepath.
func getFileHandler(conn net.Conn, req HTTPRequest, dir string) {
	fileName := strings.TrimPrefix(req.Path, "/files/")
	// Security: filepath.Join prevents directory traversal attacks (e.g., ../../etc/passwd)
	fullPath := filepath.Join(dir, fileName)

	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		sendResponse(conn, "404 Not Found", nil, "", req)
		return
	}

	headers := map[string]string{"Content-Type": "application/octet-stream"}
	sendResponse(conn, "200 OK", headers, string(fileData), req)
}

// createFileHandler handles POST "/files/{filename}"
// Demonstrates: Writing data to disk.
func createFileHandler(conn net.Conn, req HTTPRequest, dir string) {
	fileName := strings.TrimPrefix(req.Path, "/files/")
	fullPath := filepath.Join(dir, fileName)

	// Write the Request Body to the file
	err := os.WriteFile(fullPath, []byte(req.Body), 0644)
	if err != nil {
		sendResponse(conn, "500 Internal Server Error", nil, "", req)
		return
	}
	sendResponse(conn, "201 Created", nil, "", req)
}

// =================================================================================
// PART 3: MAIN SERVER SETUP
// =================================================================================

func main() {
	// Parse command line flags
	dir := flag.String("directory", ".", "Directory to serve files from")
	flag.Parse()

	// --- SETUP ROUTER ---
	// We create the router and "wire up" the paths to the functions.
	// This is often called "Registration" or "Bootstrapping".
	router := NewRouter()
	
	router.Get("/", rootHandler)
	router.Get("/echo/", echoHandler)       
	router.Get("/user-agent", userAgentHandler)
	router.Get("/files/", getFileHandler)
	router.Post("/files/", createFileHandler)
	// Handle CORS preflight for all paths ("/" is treated as a prefix match).
	router.Register("OPTIONS", "/", optionsHandler)

	fmt.Println("Server running on port 4221...")
	
	// Create TCP Listener
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()

	// Main Loop: Accept connections forever
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		// Concurrency: Handle every connection in a separate goroutine
		go handleConnection(conn, router, *dir)
	}
}

// =================================================================================
// PART 4: LOW-LEVEL NETWORKING HELPER
// This function bridges the gap between raw TCP bytes and our Router logic.
// =================================================================================

func handleConnection(conn net.Conn, router *Router, dir string) {
	// Ensure connection closes when we are done
	defer conn.Close()

	// Loop to support "Keep-Alive" (Persistent Connections)
	for {
		// 1. Read Raw Bytes
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		
		// Handle EOF (Client closed connection) or Errors
		if err == io.EOF || n == 0 { break }
		if err != nil { break }

		rawRequest := string(buf[:n])
		
		// 2. Parse Raw String into HTTPRequest Struct
		req, isValid := parseRequest(rawRequest)
		if !isValid { continue } // Skip malformed requests

		// 3. Delegate work to the Router
		// We pass the parsed request, not the raw bytes
		router.ServeRequest(conn, req, dir)

		// 4. Respect "Connection: close"
		// If the client asks to close, we break the loop, triggering the defer conn.Close()
		if val, ok := req.Headers["Connection"]; ok && val == "close" {
			break
		}
	}
}

// parseRequest converts a raw HTTP string into a usable struct.
// Raw Example: "GET /index.html HTTP/1.1\r\nHost: localhost\r\n\r\n"
func parseRequest(raw string) (HTTPRequest, bool) {
	// Split Header section from Body section (separated by double newline)
	parts := strings.Split(raw, "\r\n\r\n")
	headerPart := parts[0]
	body := ""
	if len(parts) > 1 {
		body = parts[1]
	}

	lines := strings.Split(headerPart, "\r\n")
	requestLine := strings.Split(lines[0], " ")
	
	// Valid request line must have at least Method and Path (e.g., "GET /")
	if len(requestLine) < 2 {
		return HTTPRequest{}, false
	}

	req := HTTPRequest{
		Method:  requestLine[0],
		Path:    requestLine[1],
		Headers: make(map[string]string),
		Body:    body,
	}

	// Parse Headers
	for _, line := range lines[1:] {
		// Look for the ": " separator
		if colonIdx := strings.Index(line, ": "); colonIdx != -1 {
			key := line[:colonIdx]
			val := line[colonIdx+2:]
			req.Headers[key] = val
		}
	}
	return req, true
}

// sendResponse formats and writes the HTTP response back to the client.
func sendResponse(conn net.Conn, status string, headers map[string]string, body string, req HTTPRequest) {
	// Ensure headers map is non-nil so we can safely add CORS headers.
	if headers == nil {
		headers = make(map[string]string)
	}

	// Basic CORS headers so the frontend (running on a different port) can
	// access responses from this server.
	if _, exists := headers["Access-Control-Allow-Origin"]; !exists {
		headers["Access-Control-Allow-Origin"] = "*"
	}
	if _, exists := headers["Access-Control-Allow-Methods"]; !exists {
		headers["Access-Control-Allow-Methods"] = "GET, POST, OPTIONS"
	}
	if _, exists := headers["Access-Control-Allow-Headers"]; !exists {
		headers["Access-Control-Allow-Headers"] = "Content-Type, Accept, Accept-Encoding, X-Requested-With"
	}

	// Start with the Status Line
	response := []string{fmt.Sprintf("HTTP/1.1 %s", status)}
	
	// Add custom headers (Content-Type, Content-Encoding, etc.)
	for k, v := range headers {
		response = append(response, fmt.Sprintf("%s: %s", k, v))
	}
	
	// Always calculate Content-Length automatically
	response = append(response, fmt.Sprintf("Content-Length: %d", len(body)))
	
	// If the client asked to close, confirm it in our response headers
	if val, ok := req.Headers["Connection"]; ok && val == "close" {
		response = append(response, "Connection: close")
	}
	
	// Combine Headers and Body with the mandatory blank line in between
	finalResp := strings.Join(response, "\r\n") + "\r\n\r\n" + body
	
	conn.Write([]byte(finalResp))
}