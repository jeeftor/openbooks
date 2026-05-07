#!/bin/bash

echo "Building openbooks-abs frontend."
cd server/app
npm install
npm run build
cd ../../cmd/openbooks

echo "Building binaries for various platforms."
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../../build/openbooks-abs.exe
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ../../build/openbooks-abs_mac
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ../../build/openbooks-abs_mac_arm
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../build/openbooks-abs_linux
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ../../build/openbooks-abs_linux_arm
