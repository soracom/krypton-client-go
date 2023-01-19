#!/usr/bin/env bash

VERSION=$1
if [ -z "$1" ]; then
  VERSION="0.0.0"
  echo "Version number (e.g. 1.2.3) is not specified. Using $VERSION as the default version number"
fi

set -Eeuo pipefail
d=$( cd "$(dirname "$0" )"; cd ..; pwd -P )
source "$d/scripts/common.sh"

# parameters
exe=krypton-cli
dist="$d/cmd/$exe/dist"

gopath_cache_host="$d/.cache/docker/gopath"
gopath_cache_rpi_host="$d/.cache/docker/gopath_rpi"
ecr_endpoint="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

tmpdir="$( mktemp -d )"
trap cleanup EXIT
cleanup() {
  rm -rf "$tmpdir"
  echo "Deleted temporary working directory: '$tmpdir'"
}

mkdir -p "$d/.cache"

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
  if [ "$( uname -s )" == "Linux" ]; then
    check_command_available docker
  fi
}

set -e # aborting if any commands below exit with non-zero code

# https://github.com/niemeyer/gopkg/issues/50
git config --global http.https://gopkg.in.followRedirects true

build_for_windows() {
  for arch in amd64 386; do
    echo "Building for Windows ($arch)..."
    GOOS=windows GOARCH=$arch go build -o "$tmpdir/$VERSION/${exe}.exe" -ldflags="-X main.Version=$VERSION"
    cd "$tmpdir/$VERSION" && zip "$dist/$VERSION/${exe}_${VERSION}_windows_${arch}.zip" "${exe}.exe"; cd -
  done
}

build_for_mac() {
  if [ "$( uname -s )" != "Darwin" ]; then
    echo "Building an executable for Mac can be done only on Mac"
    return
  fi
  echo "Building for macOS ..."

  arch=arm64
  if [ "$( uname -m )" == "x86_64" ]; then
    arch=amd64
  fi

  td="$tmpdir/$VERSION/${exe}_${VERSION}_darwin_${arch}"
  GOOS=darwin GOARCH=$arch go build -o "$td/${exe}" -ldflags="-X main.Version=$VERSION"
  cd "$td" && zip -r "$dist/$VERSION/${exe}_${VERSION}_darwin_${arch}.zip" "$exe"; cd -
  cp "$td/$exe" "$dist/$VERSION/${exe}_${VERSION}_darwin_${arch}"
}

build_for_linux() {
  if [ "$( uname -s )" != "Linux" ]; then
    echo "Building an executable for Linux can be done only on Linux"
    return
  fi

  arch=$1
  echo "Building for Linux ($arch)..."

  tdc="dist/$VERSION/${exe}_${VERSION}_linux_${arch}" # output directory in container
  tdh="$d/cmd/$exe/$tdc" # output directory on host
  container="${ecr_endpoint}/krypton-cli-build:latest"
  docker run -it --rm \
    -v "$d:/src" \
    -v "$gopath_cache_host:/go" \
    -u "$(id -u):$(id -g)" \
    "$container" \
    sh -c "cd /src/cmd/krypton-cli && GOOS=linux GOARCH=$arch go build -o $tdc/$exe -ldflags='-X main.Version=$VERSION'"
  cd "$tdh" && tar czvf "$dist/$VERSION/${exe}_${VERSION}_linux_${arch}.tar.gz" -- *; cd -
}

build_for_raspberry_pi() {
  if [ "$( uname -s )" != "Linux" ]; then
    echo "Building an executable for Raspberry Pi can be done only on Linux"
    return
  fi

  arch=$1
  echo "Building for Raspberry Pi ($arch)..."

  tdc="dist/$VERSION/${exe}_${VERSION}_linux_${arch}" # output directory in container
  tdh="$d/cmd/$exe/$tdc" # output directory on host

  case "$arch" in
    "arm")
      container="${ecr_endpoint}/krypton-cli-build-raspi:latest"
      ;;
    "arm64")
      container="${ecr_endpoint}/krypton-cli-build-raspi64:latest"
      ;;
    *)
      echo "unknown arch for raspi: $arch"
      exit 1
      ;;
  esac
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
  docker run -it --rm \
    -v "$d:/src" \
    -v "$gopath_cache_rpi_host:/go" \
    -u "$(id -u):$(id -g)" \
    "$container" \
    sh -c "cd /src/cmd/krypton-cli && GOOS=linux GOARCH=$arch go build -o $tdc/$exe -ldflags='-X main.Version=$VERSION'"
  cd "$tdh" && tar czvf "$dist/$VERSION/${exe}_${VERSION}_linux_${arch}.tar.gz" -- *; cd -
}

if [ "$( uname -s )" == "Linux" ]; then
  progress "logging in to AWS ECR ..."
  aws ecr get-login-password --profile "$AWS_PROFILE" --region "$AWS_REGION" | docker login --username AWS --password-stdin "$ecr_endpoint"
fi

progress "Build krypton-cli executables"
pushd "$d/cmd/krypton-cli" > /dev/null
echo "Building artifacts ..."
rm -rf "$d/cmd/krypton-cli/dist/"
mkdir -p "$tmpdir/$VERSION"
mkdir -p "$dist/$VERSION"

build_for_mac
build_for_linux amd64
build_for_raspberry_pi arm
build_for_raspberry_pi arm64
build_for_windows

popd > /dev/null
