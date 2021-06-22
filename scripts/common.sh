#!/usr/bin/env bash

# constants
export RED="\\033[1;31m"
export GREEN="\\033[1;32m"
export LIGHT_BLUE="\\033[1;94m"
export RESET="\\033[0m"


progress() {
  echo
  echo -e "$LIGHT_BLUE$*$RESET"
}
