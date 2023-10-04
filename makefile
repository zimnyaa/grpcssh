

.PHONY: all

all:
	protoc --go_out=. grpctun/grpctun.proto
	protoc --go-grpc_out=. grpctun/grpctun.proto

	go build -o sserver ./server/
	go build -o sclient ./client/