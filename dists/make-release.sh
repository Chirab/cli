#!/bin/bash

[ "$DEBUG" = "1" ] && set -x
set -e
set -x

VERSION=""

while getopts :v:b OPT; do
  case $OPT in
    v)
      VERSION=$OPTARG
      ;;
  esac
done

if [ -z $VERSION ] ; then
  echo "Usage: $0 -v <version>" >&2
  exit 1
fi

bin_dir="bin/$VERSION"
mkdir -p $bin_dir

function build_for() {
  local os=$1
  local archive_type=$2

  for arch in amd64 arm64 386 ; do
    if [ "$os" = "darwin" ] && [ "$arch" = "386" ] ; then
      continue
    fi

    pushd scalingo

    [ -e "./scalingo" ] && rm ./scalingo
    [ -e "./scalingo.exe" ] && rm ./scalingo.exe
    GOOS=$os GOARCH=$arch go build -ldflags " \
      -X main.buildstamp=$(date -u '+%Y-%m-%d_%I:%M:%S%p') \
      -X main.githash=$(git rev-parse HEAD) \
      -X main.VERSION=$VERSION"

    release_dir="scalingo_${VERSION}_${os}_${arch}"
    archive_dir="$bin_dir/$release_dir"

    popd
    mkdir -p $archive_dir

    bin="scalingo/scalingo"
    if [ "$os" = "windows" ] ; then
      bin="scalingo/scalingo.exe"
    fi
    cp $bin $archive_dir
    cp README.md $archive_dir
    cp LICENSE $archive_dir

    pushd $bin_dir
    if [ "$archive_type" = "tarball" ] ; then
      tar czvf "${release_dir}.tar.gz" "$release_dir"
    else
      zip "${release_dir}.zip" $(find "${release_dir}")
    fi
    popd
  done
}

if uname -a | grep -iq Linux ; then
  build_for "linux" "tarball"
  build_for "freebsd"
  build_for "openbsd"
  build_for "darwin"
  build_for "windows"
fi
if uname -a | grep -iq Darwin ; then
  build_for "darwin"
fi
if uname -a | grep -iq Mingw ; then
  build_for "windows"
fi
if uname -a | grep -iq Cygwin ; then
  build_for windows
fi

exit 1
