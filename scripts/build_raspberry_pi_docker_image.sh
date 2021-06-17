#!/usr/bin/env bash
# This script builds a docker container image that is imported from an official raspios image, which runs on x86_64 servers thanks to qemu.
# Inspired by http://blog.guiraudet.com/raspberrypi/2016/03/03/raspbian-image-for-docker.html
# This script also pushes the image to AWS ECR.
set -Eeuo pipefail
d=$( cd "$(dirname "$0" )"; cd ..; pwd -P )
source "$d/scripts/common.sh"

# parameters
sector_size=512
start_sector=532480
ecr_endpoint="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
mount_point="/mnt/raspios_lite_armhf"
image_zip_file="2021-05-07-raspios-buster-armhf-lite.zip"
image_path="raspios_lite_armhf/images/raspios_lite_armhf-2021-05-28/${image_zip_file}"
image_file="${image_zip_file/.zip/.img}"
container_name=raspios-lite-armv7l-runs-on-x86-64

timestamp="$( date +"%Y%m%d-%H%M%S")"
tmpdir="$( mktemp -d )"

result=1

trap cleanup EXIT
cleanup() {
  if [ $result -ne 0 ]; then
    echo
    echo -e "${RED}failed${RESET}"
    echo
  else
    echo
    echo -e "${GREEN}success${RESET}"
    echo
  fi

  set +e
  sudo rm -rf "$tmpdir"
  [ -d "$mount_point" ] && sudo umount "$mount_point"
}

progress "downloading raspberry pi os image ..."
mkdir -p "$tmpdir/zip/"
curl -L -o "$tmpdir/zip/$image_zip_file" "https://downloads.raspberrypi.org/$image_path"

progress "unzipping it ..."
(cd "$tmpdir/zip" && unzip "$image_zip_file" && ls)

echo
fdisk -l "$tmpdir/zip/$image_file"
echo

echo -n "Does the sector size equal to ${sector_size} and the Start sector for Linux partition equal to ${start_sector}? (y/n) "
read -r y

case $y in
  Y|y)
    ;;
  *)
    exit 1;
    ;;
esac

progress "mounting the image to $mount_point ..."
echo "Info: we need root access to mount the image"
sudo mkdir -p "$mount_point"
sudo mount -o loop,offset=$((sector_size * start_sector)) "$tmpdir/zip/$image_file" "$mount_point"

progress "disabling preloaded shared libraries to get everything including networking to work on x86_64 ..."
sudo mv "$mount_point/etc/ld.so.preload" "$mount_point/etc/ld.so.preload.bak"

progress "packaging ..."
sudo tar -C "$mount_point" -cf "$tmpdir/docker-image-${timestamp}-raspios-lite.tar" .

progress "importing it to docker ..."
docker import "$tmpdir/docker-image-${timestamp}-raspios-lite.tar" "${container_name}:${timestamp}"

progress "testing the image ..."
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
arch="$( docker run --rm -it "${container_name}:${timestamp}" uname -m | tr -d '\r\n' )"
if [[ "$arch" != "armv7l" ]]; then
  echo "expected armv7l but got '$arch'"
  exit 1
fi

progress "logging in to AWS ECR ..."
aws ecr get-login-password --profile "$AWS_PROFILE" --region "$AWS_REGION" | docker login --username AWS --password-stdin "$ecr_endpoint"

progress "pushing the image to the registry ..."
sha="$( docker image inspect "${container_name}:${timestamp}" -f '{{ .Id }}' )"
container_id="${sha#sha256:}"
docker tag "$container_id" "${ecr_endpoint}/${container_name}:${timestamp}"
docker tag "$container_id" "${ecr_endpoint}/${container_name}:latest"
docker push "${ecr_endpoint}/${container_name}"

result=0
