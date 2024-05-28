// main.go
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/tillknuesting/go-grpc-gateway-streaming-example/proto"
)

type GreeterServer struct {
	pb.UnimplementedGreeterServer
}

func (s *GreeterServer) SayHello(req *pb.HelloRequest, stream pb.Greeter_SayHelloServer) error {
	for i := range 1000 {
		resp := &pb.HelloResponse{
			Message: fmt.Sprintf("Hello, %s! (message %d)", req.GetName(), i+1),
		}
		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("sending on stream from SayHello: %w", err)
		}
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (g GreeterServer) mustEmbedUnimplementedGreeterServer() {
	// TODO implement me
	panic("implement me")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)

			return
		}

		next.ServeHTTP(w, r)
	})
}

type captureResponseWriter struct {
	http.ResponseWriter
	DataChan chan []byte
}

func (mw *captureResponseWriter) Write(b []byte) (int, error) {
	if len(b) > 1 {
		mw.DataChan <- b
	}
	return mw.ResponseWriter.Write(b)
}

func (mw *captureResponseWriter) Unwrap() http.ResponseWriter {
	return mw.ResponseWriter
}

var dataChanMap sync.Map // Map to store data channels by session UUID

// SessionMetadata holds the session ID and source instance ID.
type SessionMetadata struct {
	SessionUUID string `json:"session_uuid"`
	// Source instance identifier is used for network routing scenarios
	SourceInstanceID string `json:"source_instance_id"`
}

func generateSecureSessionID() string {
	generatedUUID := uuid.New().String()
	hash := sha256.Sum256([]byte(generatedUUID))
	return fmt.Sprintf("%x", hash)
}

func sseResponseStreamingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Use-SSE") == "true" {

			sessionUUID := generateSecureSessionID()
			dataChan := make(chan []byte, 1000)
			dataChanMap.Store(sessionUUID, dataChan)

			sourceInstanceID := "my_server1" // Replace with your source instance identifier

			sessionData := SessionMetadata{
				SessionUUID:      sessionUUID,
				SourceInstanceID: sourceInstanceID,
			}

			// Marshal session metadata into JSON
			responseData, err := json.Marshal(sessionData)
			if err != nil {
				http.Error(w, "Failed to generate session", http.StatusInternalServerError)
				return
			}

			// Get the underlying connection using http.Hijacker
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
				return
			}

			conn, bufw, err := hijacker.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Set the response headers
			bufw.WriteString("HTTP/1.1 200 OK\r\n")
			bufw.WriteString("Content-Type: application/json\r\n")
			bufw.WriteString("Connection: close\r\n\r\n")

			// Write the initial response
			bufw.WriteString(string(responseData) + "\n\n")
			bufw.Flush()
			conn.Close()

			sw := &captureResponseWriter{
				ResponseWriter: httptest.NewRecorder(),
				DataChan:       dataChan,
			}

			next.ServeHTTP(sw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	// Get the session UUID from the request URL
	sessionUUID := r.URL.Path[len("/sse/"):]

	// Get the data channel for the session UUID
	dataChanValue, ok := dataChanMap.Load(sessionUUID)
	if !ok {
		http.Error(w, "Invalid session UUID", http.StatusBadRequest)
		return
	}
	dataChan, ok := dataChanValue.(chan []byte)
	if !ok {
		http.Error(w, "Invalid data channel", http.StatusInternalServerError)
		return
	}

	// Set the response headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("Access-Control-Allow-Origin", "*")

	var lastTimestamp int64 = 0
	var eventIDCounter int64 = 0

	// Send the data chunks as SSE events
	for data := range dataChan {
		timestamp := time.Now().UnixNano()
		if timestamp == lastTimestamp {
			eventIDCounter++
		} else {
			eventIDCounter = 0
		}
		lastTimestamp = timestamp

		fmt.Fprintf(w, "event: output\n")
		fmt.Fprintf(w, "id: %d:%d\n", timestamp, eventIDCounter)
		fmt.Fprintf(w, "data: %s\n\n", data)

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Send the "done" event
	currentTimestamp := time.Now().UnixNano()
	if currentTimestamp == lastTimestamp {
		eventIDCounter++
	} else {
		eventIDCounter = 0
	}
	fmt.Fprintf(w, "event: done\n")
	fmt.Fprintf(w, "id: %d:%d\n", currentTimestamp, eventIDCounter)
	fmt.Fprintf(w, "data: {}\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Remove the data channel from the map when the SSE connection is closed
	dataChanMap.Delete(sessionUUID)
}

func main() {
	// Create a listener on TCP port
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Create a gRPC server object
	grpcServer := grpc.NewServer()
	// Attach the Greeter service to the server
	pb.RegisterGreeterServer(grpcServer, &GreeterServer{})
	// Serve gRPC server
	log.Println("Serving gRPC on 0.0.0.0:8080")

	go func() {
		log.Fatalln(grpcServer.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	go func() {
		// Create a new HTTP server mux
		sseMux := http.NewServeMux()

		// Register the SSE handler at the "/sse/" endpoint
		sseMux.HandleFunc("/sse/", sseHandler)

		// Create a new HTTP server for SSE
		sseServer := &http.Server{
			Addr:              ":8091",
			Handler:           sseMux,
			ReadHeaderTimeout: time.Second,
		}

		log.Printf("SSE server listening on %s", sseServer.Addr)
		if err := sseServer.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start SSE server: %v", err)
		}
	}()

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = pb.RegisterGreeterHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	// Apply CORS middleware to the gRPC-Gateway mux
	handler2 := corsMiddleware(gwmux)

	// Wrap the mux with the logging middleware
	handler := sseResponseStreamingMiddleware(handler2)

	// Start the gRPC-Gateway server with CORS support
	gwServer := &http.Server{
		Addr:              ":8090",
		Handler:           handler,
		ReadHeaderTimeout: time.Second,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())
}
