// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/tillknuesting/go-grpc-gateway-streaming-example/proto"
)

type GreeterServer struct {
	pb.UnimplementedGreeterServer
}

func (s *GreeterServer) SayHello(req *pb.HelloRequest, stream pb.Greeter_SayHelloServer) error {
	for i := range 100 {
		resp := &pb.HelloResponse{
			Message: fmt.Sprintf("Hello, %s! (message %d)", req.GetName(), i+1),
		}
		if err := stream.Send(resp); err != nil {
			return fmt.Errorf("sending on stream from SayHello: %w", err)
		}

		time.Sleep(time.Second)
	}
	return nil
}

func (g GreeterServer) mustEmbedUnimplementedGreeterServer() {
	//TODO implement me
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

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = pb.RegisterGreeterHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	// Apply CORS middleware to the gRPC-Gateway mux
	handler := corsMiddleware(gwmux)

	// Start the gRPC-Gateway server with CORS support
	gwServer := &http.Server{
		Addr:              ":8090",
		Handler:           handler,
		ReadHeaderTimeout: time.Second,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())
}
