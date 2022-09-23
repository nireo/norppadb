.PHONY: compile
compile:
	protoc store/*.proto \
		--go_out=. \
		--go_opt=paths=source_relative \
		--proto_path=.

.PHONY: check
check:
	go vet ./...
	staticcheck ./...
