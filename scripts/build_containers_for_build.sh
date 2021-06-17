#!/usr/bin/env bash
set -Eeuo pipefail
d=$( cd "$(dirname "$0" )"; cd ..; pwd -P )
source "$d/scripts/common.sh"

# parameters
go_version=1.16.5
go_package="go${go_version}.linux-armv6l.tar.gz"
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
  rm -f "$d/scripts/$go_package"
}

progress "build image for linux armv6l (for Raspberry Pi)"
container_name=krypton-cli-build-raspi
curl -L -o "$d/scripts/$go_package" "https://golang.org/dl/${go_package}"  # downloading the huge file in raspi container is way slow, so downloading it here instead.
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker build -t "${container_name}:${timestamp}" -f "$d/scripts/Dockerfile-raspi" "$d/scripts"

progress "pushing the image to the registry ..."
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:${timestamp}"
docker tag "${container_name}:${timestamp}" "${ecr_endpoint}/${container_name}:latest"
docker push "${ecr_endpoint}/${container_name}"

