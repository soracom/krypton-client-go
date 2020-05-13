#!/bin/bash
d=$( cd "$(dirname "$0" )"; cd ..; pwd -P )
exe=krypton-cli
tmpdir="$( mktemp -d )"
dist="$d/cmd/$exe/dist"

function cleanup() {
  rm -rf "$tmpdir"
  echo "Deleted temporary working directory: '$tmpdir'"
}

trap cleanup EXIT

: "Check if shell scripts are healthy" && {
  command -v shellcheck > /dev/null 2>&1 && {
    shellcheck -e SC2164 "$d/scripts/"*.sh
  }
}

check_command_available() {
  local cmd=$1
  command -v "$cmd" > /dev/null 2>&1 || {
    echo "\`$cmd\` is required."
    exit 1
  }
}

: "Check if required commands for build are available" && {
  check_command_available go
  check_command_available git
  check_command_available docker
}

set -e # aborting if any commands below exit with non-zero code

VERSION=$1
if [ -z "$1" ]; then
  VERSION="0.0.0"
  echo "Version number (e.g. 1.2.3) is not specified. Using $VERSION as the default version number"
fi

# https://github.com/niemeyer/gopkg/issues/50
git config --global http.https://gopkg.in.followRedirects true

function build_for_windows() {
  for arch in amd64 386; do
    GOOS=windows GOARCH=$arch go build -o "$tmpdir/$VERSION/${exe}.exe" -ldflags="-X main.Version=$VERSION"
    cd "$tmpdir/$VERSION" && zip "$dist/$VERSION/${exe}_${VERSION}_windows_${arch}.zip" "${exe}.exe"; cd -
  done
}

function build_for_mac() {
  if [[ "$( uname -s )" != "Darwin" ]]; then
    echo "Building an executable for Mac can be don only on Mac"
    return
  fi

  arch=$1
  td="$tmpdir/$VERSION/${exe}_${VERSION}_darwin_${arch}"
  GOOS=darwin GOARCH=$arch go build -o "$td/${exe}" -ldflags="-X main.Version=$VERSION"
  cd "$td" && zip -r "$dist/$VERSION/${exe}_${VERSION}_darwin_${arch}.zip" "$exe"; cd -
  cp "$td/$exe" "$dist/$VERSION/${exe}_${VERSION}_darwin_${arch}"
}

function build_for_linux() {
  arch=$1
  tdc="dist/$VERSION/${exe}_${VERSION}_linux_${arch}" # output directory in container
  tdh="$d/cmd/$exe/$tdc" # output directory on host
  container=krypton-cli-build
  docker build -t $container "$d/scripts"
  docker run -it --rm -v "$d:/src" -v "$GOPATH:/go" -u "$(id -u):$(id -g)" $container sh -c \
    "cd /src/cmd/krypton-cli && GOOS=linux GOARCH=$arch go build -o $tdc/$exe -ldflags='-X main.Version=$VERSION'"
  cd "$tdh" && tar czvf "$dist/$VERSION/${exe}_${VERSION}_linux_${arch}.tar.gz" -- *; cd -
  rm -rf "$tdh"
}

: "Build krypton-cli executables" && {
    pushd "$d/cmd/krypton-cli" > /dev/null
    echo "Building artifacts ..."
    rm -rf "$d/cmd/krypton-cli/dist/"
    mkdir -p "$tmpdir/$VERSION"
    mkdir -p "$dist/$VERSION"

    build_for_mac amd64
    build_for_linux amd64
    build_for_windows

    popd > /dev/null
}
