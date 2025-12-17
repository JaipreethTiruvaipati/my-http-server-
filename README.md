[![progress-banner](https://backend.codecrafters.io/progress/http-server/d37e4781-e748-4b7a-98b4-d776c59513fa)](https://app.codecrafters.io/users/codecrafters-bot?r=2qF)

# High-Performance HTTP Server in Go

A lightweight, multi-threaded HTTP/1.1 server built from scratch in Go. This project implements core HTTP protocol features including concurrent connection handling, dynamic routing, file persistence, compression, and connection management without relying on external web frameworks.

## 🚀 Features

* **Concurrency:** Handles multiple simultaneous clients using Go routines (green threads).
* **Custom HTTP Parsing:** Manually parses HTTP Request lines, Headers, and Bodies from raw byte streams.
* **Dynamic Routing:** Supports multiple endpoints (`/`, `/echo`, `/user-agent`, `/files`).
* **File I/O:**
    * `GET` requests to serve static files.
    * `POST` requests to upload and save files to disk.
    * Configurable storage directory via command-line flags.
* **Compression:** Supports `gzip` compression for bandwidth optimization (Content Negotiation).
* **Connection Management:**
    * Implements **Persistent Connections (Keep-Alive)** by default for performance.
    * Supports **Graceful Teardown** via the `Connection: close` header.

## 🛠️ Tech Stack

* **Language:** Go (Golang)
* **Standard Libraries:** `net` (TCP), `io`, `os`, `compress/gzip`, `bytes`, `strings`, `flag`.
* **Architecture:** TCP Listener -> Concurrent Goroutine Workers -> Request Parser -> Route Handler.

## 🏃‍♂️ Getting Started

### Prerequisites
* Go 1.19+ installed.

### Installation
Clone the repository (or copy the code):
```bash
git clone [https://github.com/JaipreethTiruvaipati/my-http-server-](https://github.com/JaipreethTiruvaipati/my-http-server-)
