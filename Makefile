
.PHONY: generate
generate:
	protoc --go_out=. --go_opt=paths=source_relative \
		--stream-rpc_out=. --stream-rpc_opt=paths=source_relative \
		examples/basic/calculator/proto/calculator.proto

.PHONY: build
build: generate
	go build ./... 