#!/usr/bin/env bash
set -Eeuo pipefail
d=$( cd "$(dirname "$0" )"; cd ..; pwd -P )
source "$d/scripts/common.sh"

# parameters
go_version=1.19.5
go_package_arm32="go${go_version}.linux-armv6l.tar.gz"
go_package_arm64="go${go_version}.linux-arm64.tar.gz"
ecr_endpoint="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
timestamp="$( date +"%Y%m%d-%H%M%S")"

progress "logging in to AWS ECR ..."
aws ecr get-login-password --profile "$AWS_PROFILE" --region "$AWS_REGION" | docker login --username AWS --password-stdin "$ecr_endpoint"

progress "build image for linux amd64"
container_name=krypton-cli-build
docker build -t "${container_name}:${timestamp}" -f "$d/scripts/Dockerfile" "$d/scripts"

progress "pushing the image to the registry ..."
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:${timestamp}"
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:latest"
docker push "${ecr_endpoint}/${container_name}"

trap cleanup EXIT

cleanup() {
  rm -f "$d/scripts/$go_package_arm32"
  rm -f "$d/scripts/$go_package_arm64"
}

progress "build image for linux armv6l (for Raspberry Pi)"
container_name=krypton-cli-build-raspi
curl -L -o "$d/scripts/$go_package_arm32" "https://golang.org/dl/${go_package_arm32}"  # downloading the huge file in raspi container is way slow, so downloading it here instead.
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker build -t "${container_name}:${timestamp}" -f "$d/scripts/Dockerfile-raspi" "$d/scripts"

progress "build image for linux aarch64 (for Raspberry Pi 64bit)"
container_name=krypton-cli-build-raspi64
curl -L -o "$d/scripts/$go_package_arm64" "https://golang.org/dl/${go_package_arm64}"  # downloading the huge file in raspi container is way slow, so downloading it here instead.
docker build -t "${container_name}:${timestamp}" -f "$d/scripts/Dockerfile-raspi64" "$d/scripts"

progress "pushing the image to the registry ..."
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:${timestamp}"
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:latest"
docker push "${ecr_endpoint}/${container_name}"

