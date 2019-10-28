#!/bin/bash
protoc -I ./localinterface --go_out=plugins=grpc:./localinterface cli.proto

## prereq:
# go get -u github.com/golang/protobuf/protoc-gen-go