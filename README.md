# go-grpc-gateway-streaming-example
This demo showcases how to use gRPC streaming and gRPC-Gateway with a frontend that runs in a browser.
In addition, there is a middleware that makes it possible to receive the streaming responses as SSE on a separate endpoint.

`go run server.go`

To test default streaming with grpc-gateway:

open testing/index.html in browser and type in a message.

or 

`curl -X GET "http://localhost:8090/v1/hello?name=John" -H "Accept: application/json"`


to test SSE:

`curl -X GET -H "X-Use-SSE: true"  "http://localhost:8090/v1/hello?name=John" -H "Accept: application/json"`

example output:
`{"session_uuid":"3a4733bc352ee63a009c7530c952350a5dd3ae1cd136a194f8647c3384923148","source_instance_id":"test-server-1"}`

Now you can use the session_uuid in the index-sse.html (a react frontend) to get the SSE stream.

or 

use the session_uuid to get the SSE stream:

`curl -v -N -H "Accept: text/event-stream" http://localhost:8091/sse/<session_uuid>`
