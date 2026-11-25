#!/bin/sh
set -e
V=${1:?usage: ./build.sh 1.2.3}
export PATH=$PATH:$(go env GOPATH)/bin
command -v go-winres >/dev/null || go install github.com/tc-hib/go-winres@latest
go-winres simply --arch amd64 --icon icon.png --manifest gui --product-name Seismo --file-description "Seismo market event radar" --product-version "$V" --file-version "$V" --original-filename Seismo.exe
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w -X main.version=$V" -o Seismo.ex_ .
echo "$V" > version.txt
echo "built Seismo.ex_ v$V"
