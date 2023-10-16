#!/bin/bash

rootDir=$(dirname $0)
pushd ${rootDir}/web
npm install
npm run build
popd

go mod download
go build -ldflags "-s -w" -o one-api