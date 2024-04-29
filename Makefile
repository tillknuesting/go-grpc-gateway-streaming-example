# Makefile

PROTO_DIR := proto
THIRD_PARTY_DIR := third_party
GOOGLEAPIS_URL := https://raw.githubusercontent.com/googleapis/googleapis/master/google/api
PROTOBUF_URL := https://raw.githubusercontent.com/protocolbuffers/protobuf/master/src/google/protobuf

.PHONY: all clean generate

all: generate

$(THIRD_PARTY_DIR)/google/api/annotations.proto:
	mkdir -p $(THIRD_PARTY_DIR)/google/api
	curl -o $(THIRD_PARTY_DIR)/google/api/annotations.proto $(GOOGLEAPIS_URL)/annotations.proto

$(THIRD_PARTY_DIR)/google/api/http.proto:
	mkdir -p $(THIRD_PARTY_DIR)/google/api
	curl -o $(THIRD_PARTY_DIR)/google/api/http.proto $(GOOGLEAPIS_URL)/http.proto

$(THIRD_PARTY_DIR)/google/protobuf/descriptor.proto:
	mkdir -p $(THIRD_PARTY_DIR)/google/protobuf
	curl -o $(THIRD_PARTY_DIR)/google/protobuf/descriptor.proto $(PROTOBUF_URL)/descriptor.proto

generate: $(THIRD_PARTY_DIR)/google/api/annotations.proto $(THIRD_PARTY_DIR)/google/api/http.proto $(THIRD_PARTY_DIR)/google/protobuf/descriptor.proto
	protoc -I $(PROTO_DIR) -I $(THIRD_PARTY_DIR) \
		--go_out $(PROTO_DIR) --go_opt paths=source_relative \
		--go-grpc_out $(PROTO_DIR) --go-grpc_opt paths=source_relative \
		--grpc-gateway_out $(PROTO_DIR) --grpc-gateway_opt paths=source_relative \
		--openapiv2_out $(PROTO_DIR) \
		$(PROTO_DIR)/greeter.proto

clean:
	rm -rf $(THIRD_PARTY_DIR)/google/api $(THIRD_PARTY_DIR)/google/protobuf
	rm -f $(PROTO_DIR)/*.pb.go $(PROTO_DIR)/*.pb.gw.go $(PROTO_DIR)/*.swagger.json