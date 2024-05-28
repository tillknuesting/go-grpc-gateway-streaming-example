# go-grpc-gateway-streaming-example

This demo showcases how to use gRPC streaming and gRPC-Gateway with a frontend that runs in a browser. In addition, there is a middleware that makes it possible to receive the streaming responses as Server-Sent Events (SSE) on a separate endpoint.

## Features

- gRPC streaming server
- gRPC-Gateway for HTTP/JSON endpoints
- SSE middleware for streaming responses to the browser

## Getting Started

1. Clone the repository:
   ```
   git clone https://github.com/tillknuesting/go-grpc-gateway-streaming-example.git
   ```

2. Run the server:
   ```
   go run server.go
   ```

## Testing

### Default Streaming with gRPC-Gateway

- Open `testing/index.html` in a browser and type in a message.
- Or use curl:
  ```
  curl -X GET "http://localhost:8090/v1/hello?name=John" -H "Accept: application/json"
  ```

### SSE Streaming

- Use curl to initiate an SSE stream:
  ```
  curl -X GET -H "X-Use-SSE: true"  "http://localhost:8090/v1/hello?name=John" -H "Accept: application/json"
  ```
  Example output:
  ```
  {"session_uuid":"3a4733bc352ee63a009c7530c952350a5dd3ae1cd136a194f8647c3384923148","source_instance_id":"test-server-1"}
  ```

- Use the `session_uuid` in the `index-sse.html` (a React frontend) to get the SSE stream.
- Or use curl with the `session_uuid` to get the SSE stream:
  ```
  curl -v -N -H "Accept: text/event-stream" http://localhost:8091/sse/<session_uuid>
  ```

## High-Level Design of the Middleware

The middleware in this example serves two main purposes:

2. SSE Response Streaming Middleware:
    - The `sseResponseStreamingMiddleware` function intercepts requests with the `X-Use-SSE` header set to `true`.
    - It generates a secure session UUID and creates a data channel for that session.
    - The session metadata, including the session UUID and source instance ID, is sent back to the client immediately.
    - The middleware then continues calling the gRPC-Gateway endpoint and streams the data to the SSE handler using the session UUID.
    - The `captureResponseWriter` is used to capture the response data and send it through the data channel.

3. SSE Handler:
    - The `sseHandler` function handles the SSE connections.
    - It retrieves the data channel based on the session UUID from the request URL.
    - It sets the appropriate headers for SSE and sends the data chunks as SSE events to the client.
    - When the streaming is complete, it sends a "done" event and closes the connection.
    - The data channel is removed from the map when the SSE connection is closed.

The middleware allows clients to initiate an SSE stream by sending a request with the `X-Use-SSE` header. The client receives a session UUID, which can be used to establish the SSE connection and receive the streaming data.