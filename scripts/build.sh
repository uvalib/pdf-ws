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

echo "git directory: [$gitdir]"
# sanity checks
[ ! -d "$srcdir" ] && die "missing source directory: [$srcdir]"
[ ! -d "$cfgdir" ] && die "missing configuration directory: [$cfgdir]"
[ ! -d "$webdir" ] && die "missing web directory: [$webdir]"

# clear out any existing dist files
echo -n "removing old distribution files... "
[ -d "$distdir" ] && rm -rf "$distdir"
echo "OK"

# ugly prep work
echo -n "setting up go environment... "
rm -f src cmd/pdf-ws/vendor
ln -sf cmd src
ln -sf ../../vendor cmd/pdf-ws/vendor
echo "OK"

# change to source directory and build app
#cd "$srcdir" || exit 1

echo -n "building app... "
#env GOOS=linux go build -o "${distdir}/pdf-ws.linux" || die "build failed"
GOPATH="$gitdir" CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o "${distdir}/pdf-ws.linux" cmd/pdf-ws/*.go || die "build failed"
echo "OK"

echo -n "copying assets... "
cp -f "${cfgdir}/config.yml.template" "${distdir}/config.yml"
cp -f "${webdir}/index.html" "${distdir}/index.html"
echo "OK"

echo
echo "dist files:"
echo

ls -lF "$distdir"
