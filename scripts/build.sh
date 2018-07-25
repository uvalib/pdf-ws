#!/bin/bash

function die ()
{
	echo "error: $@"
	exit 1
}

# change to git project directory directly above this script's location
echo -n "changing to git base directory... "
basedir="$(dirname "$0")/.."
cd "$basedir" || die "could not cd to directory: [$basedir]"
echo "OK"

# directory definitions
gitdir="$PWD"
srcdir="${gitdir}/cmd/pdf-ws"
webdir="${gitdir}/web"
distdir="${gitdir}/dist"

# sanity checks
echo -n "running sanity checks... "
[ ! -d "$srcdir" ] && die "missing source directory: [$srcdir]"
[ ! -d "$webdir" ] && die "missing web directory: [$webdir]"
echo "OK"

# clear out any existing dist files
echo -n "removing old distribution files... "
[ -d "$distdir" ] && rm -rf "$distdir"
echo "OK"

# kludgy prep work
echo -n "setting up go environment... "
rm -f src cmd/pdf-ws/vendor
ln -s cmd src
ln -s ../../vendor cmd/pdf-ws/vendor
echo "OK"

echo -n "building app... "
#env GOOS=linux go build -o "${distdir}/pdf-ws.linux" || die "build failed"
GOPATH="$gitdir" GOOS=linux GOARCH=amd64 go build -a -o "${distdir}/pdf-ws.linux" cmd/pdf-ws/*.go || die "build failed"
echo "OK"

echo -n "copying assets... "
cp -f "${webdir}/index.html" "${distdir}/index.html"
echo "OK"

echo -n "cleaning up... "
rm -f src cmd/pdf-ws/vendor
echo "OK"

echo
echo "dist files:"
echo

ls -lF "$distdir"
