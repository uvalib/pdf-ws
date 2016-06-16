#!/bin/bash
env GOOS=linux go build -o dist/pdf-ws.linux
cp config.yml.template dist/config.yml
