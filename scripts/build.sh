#!/bin/bash

# get git project directory relative to this script's location,
# so that this script can be run from anywhere
basedir="$(dirname "$0")/.."
srcdir="${basedir}/cmd/pdf-ws"
distdir="${basedir}/dist"
cfgdir="${basedir}/configs"
webdir="${basedir}/web"

# clear out any existing files
[ -d "$distdir" ] && rm -rf "$distdir"

# change to source directory and build app
cd "$srcdir" || exit 1

env GOOS=linux go build -o "${distdir}/pdf-ws.linux"
cp -f "${cfgdir}/config.yml.template" "${distdir}/config.yml"
cp -f "${webdir}/index.html" "${distdir}/index.html"
