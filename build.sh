#!/bin/sh

cd messenger
GOOS=linux GOARCH=amd64 go build
cd ..
cd scheduler
GOOS=linux GOARCH=amd64 go build
cd ..
