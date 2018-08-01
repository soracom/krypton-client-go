#!/bin/bash
d="$( cd "$( dirname "$0" )"; cd ..; pwd )"
set -e

VERSION=$1
if [ -z "$1" ]; then
  echo "Version number (e.g. 1.2.3) must be specified. Abort."
  exit 1
fi

pushd "$d/cmd/krypton-cli" >/dev/null 2>&1
ghr --prerelease --replace -u soracom -r krypton-client-go "v$VERSION" "$d/cmd/krypton-cli/dist/$VERSION/"
popd >/dev/null 2>&1

#echo
#echo "Please run \`update-homebrew-formula.sh\` as soon as the release gets published"
#echo