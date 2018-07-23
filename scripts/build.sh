#!/bin/bash

function die ()
{
	echo "error: $@"
	exit 1
}

# change to git project directory directly above this script's location
basedir="$(dirname "$0")/.."
cd "$basedir" || die "could not cd to directory: [$basedir]"

gitdir="$(pwd)"
srcdir="${gitdir}/cmd/pdf-ws"
cfgdir="${gitdir}/configs"
webdir="${gitdir}/web"
distdir="${gitdir}/dist"

# sanity checks
[ ! -d "$srcdir" ] && die "missing source directory: [$srcdir]"
[ ! -d "$cfgdir" ] && die "missing configuration directory: [$cfgdir]"
[ ! -d "$webdir" ] && die "missing web directory: [$webdir]"

# clear out any existing dist files
[ -d "$distdir" ] && rm -rf "$distdir"

# change to source directory and build app
cd "$srcdir" || exit 1

echo -n "building app... "
env GOOS=linux go build -o "${distdir}/pdf-ws.linux" || die "build failed"
echo "OK"

echo -n "copying assets..."
cp -f "${cfgdir}/config.yml.template" "${distdir}/config.yml"
cp -f "${webdir}/index.html" "${distdir}/index.html"
echo "OK"

echo
echo "dist files:"
echo

ls -lF "$distdir"
