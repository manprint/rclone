#!/usr/bin/env bash

# Installs the newest rclone webui release (pre-releases included) from
# https://github.com/manprint/rclone/releases
#
# Usage: sudo -v ; curl -fsSL https://raw.githubusercontent.com/manprint/rclone/main/webui/install.sh | sudo bash [-s <tag>]
#
# error codes
# 0 - exited without problems
# 1 - parameters not supported were used or some unexpected error occurred
# 2 - OS not supported by this script
# 3 - installed version of rclone is up to date
# 4 - supported unzip tools are not available

set -e

repo="${RCLONE_WEBUI_REPO:-manprint/rclone}"

#when adding a tool to the list make sure to also add its corresponding command further in the script
unzip_tools_list=('unzip' '7z' 'busybox')

usage() { echo "Usage: sudo -v ; curl -fsSL https://raw.githubusercontent.com/${repo}/main/webui/install.sh | sudo bash [-s <tag>]" 1>&2; exit 1; }

if [ "$#" -gt 1 ]; then
    usage
fi
case "$1" in
  -h|--help)
    usage
    ;;
esac
tag="$1"


#create tmp directory and move to it with macOS compatibility fallback
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t 'rclone-install.XXXXXXXXXX')
cd "$tmp_dir"


#make sure unzip tool is available and choose one to work with
set +e
for tool in ${unzip_tools_list[*]}; do
    trash=$(hash "$tool" 2>>errors)
    if [ "$?" -eq 0 ]; then
        unzip_tool="$tool"
        break
    fi
done
set -e

# exit if no unzip tools available
if [ -z "$unzip_tool" ]; then
    printf "\nNone of the supported tools for extracting zip archives (${unzip_tools_list[*]}) were found. "
    printf "Please install one of them and try again.\n\n"
    exit 4
fi

# Make sure we don't create a root owned .config/rclone directory #2127
export XDG_CONFIG_HOME=config

#find the newest release; /releases lists pre-releases too, unlike /releases/latest
if [ -z "$tag" ]; then
    tag=$(curl -fsSL "https://api.github.com/repos/${repo}/releases?per_page=1" \
        | sed -nE 's/.*"tag_name": *"([^"]+)".*/\1/p' \
        | head -n 1)
    if [ -z "$tag" ]; then
        echo "Could not find the latest release of ${repo}"
        exit 1
    fi
fi

#check installed version of rclone to determine if update is necessary
version=$(rclone --version 2>>errors | head -n 1)
current_version="rclone ${tag}"

if [ "$version" = "$current_version" ]; then
    printf "\nThe latest version ${version} is already installed.\n\n"
    exit 3
fi


#detect the platform
OS="$(uname)"
case $OS in
  Linux)
    OS='linux'
    ;;
  FreeBSD)
    OS='freebsd'
    ;;
  NetBSD)
    OS='netbsd'
    ;;
  OpenBSD)
    OS='openbsd'
    ;;
  Darwin)
    OS='osx'
    binTgtDir=/usr/local/bin
    man1TgtDir=/usr/local/share/man/man1
    ;;
  SunOS)
    OS='solaris'
    echo 'OS not supported'
    exit 2
    ;;
  *)
    echo 'OS not supported'
    exit 2
    ;;
esac

OS_type="$(uname -m)"
case "$OS_type" in
  x86_64|amd64)
    OS_type='amd64'
    ;;
  i?86|x86)
    OS_type='386'
    ;;
  aarch64|arm64)
    OS_type='arm64'
    ;;
  armv7*)
    OS_type='arm-v7'
    ;;
  armv6*)
    OS_type='arm-v6'
    ;;
  arm*)
    OS_type='arm'
    ;;
  *)
    echo 'OS type not supported'
    exit 2
    ;;
esac


#download, verify and unzip
release_url="https://github.com/${repo}/releases/download/${tag}"
rclone_zip="rclone-${tag}-${OS}-${OS_type}.zip"

printf "Downloading ${rclone_zip}\n"
curl -fsSLO "${release_url}/${rclone_zip}"
curl -fsSLO "${release_url}/MD5SUMS"
grep " ${rclone_zip}\$" MD5SUMS > MD5SUMS.zip
if hash md5sum 2>/dev/null; then
    md5sum -c MD5SUMS.zip
elif hash md5 2>/dev/null; then
    [ "$(md5 -q "$rclone_zip")" = "$(cut -d ' ' -f 1 MD5SUMS.zip)" ] || { echo "Checksum mismatch for ${rclone_zip}"; exit 1; }
else
    echo 'md5sum not found. The download checksum will not be verified.'
fi

unzip_dir="tmp_unzip_dir_for_rclone"
# there should be an entry in this switch for each element of unzip_tools_list
case "$unzip_tool" in
  'unzip')
    unzip -a "$rclone_zip" -d "$unzip_dir"
    ;;
  '7z')
    7z x "$rclone_zip" "-o$unzip_dir"
    ;;
  'busybox')
    mkdir -p "$unzip_dir"
    busybox unzip "$rclone_zip" -d "$unzip_dir"
    ;;
esac

cd $unzip_dir/*

#mounting rclone to environment

case "$OS" in
  'linux')
    #binary
    cp rclone /usr/bin/rclone.new
    chmod 755 /usr/bin/rclone.new
    chown root:root /usr/bin/rclone.new
    mv /usr/bin/rclone.new /usr/bin/rclone
    #manual
    if ! [ -x "$(command -v mandb)" ]; then
        echo 'mandb not found. The rclone man docs will not be installed.'
    else
        mkdir -p /usr/local/share/man/man1
        cp rclone.1 /usr/local/share/man/man1/
        mandb
    fi
    ;;
  'freebsd'|'openbsd'|'netbsd')
    #binary
    cp rclone /usr/bin/rclone.new
    chown root:wheel /usr/bin/rclone.new
    mv /usr/bin/rclone.new /usr/bin/rclone
    #manual
    mkdir -p /usr/local/man/man1
    cp rclone.1 /usr/local/man/man1/
    makewhatis
    ;;
  'osx')
    #binary
    mkdir -m 0555 -p ${binTgtDir}
    cp rclone ${binTgtDir}/rclone.new
    mv ${binTgtDir}/rclone.new ${binTgtDir}/rclone
    chmod a=x ${binTgtDir}/rclone
    #manual
    mkdir -m 0555 -p ${man1TgtDir}
    cp rclone.1 ${man1TgtDir}
    chmod a=r ${man1TgtDir}/rclone.1
    ;;
  *)
    echo 'OS not supported'
    exit 2
esac

#update version variable post install
version=$(rclone --version 2>>errors | head -n 1)

#cleanup
rm -rf "$tmp_dir"

printf "\n${version} has successfully installed."
printf '\nNow run "rclone config" for setup, then "rclone serve webdav --addr :8000 remote:" for the web UI.\n'
printf "Check https://github.com/${repo} for more details.\n\n"
exit 0
