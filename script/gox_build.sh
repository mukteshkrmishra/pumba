#!/bin/bash

distdir=.dist
exarch="amd64 386"
oslist="linux windows darwin"
BUILD_DATE=$(date -u '+%Y/%m/%d')
BUILD_VERSION=$(git describe --tags)
CGO_ENABLE=0

gox_build() {
  rm -rf "${distdir}"
  mkdir "${distdir}"
  echo "Building" ${BUILD_VERSION} "on" ${BUILD_DATE}
  glide install
  gox -os="${oslist}" -arch="${exarch}"  -ldflags "-X main.Version=${BUILD_VERSION} -X main.BuildDate='${BUILD_DATE}'" -verbose -output=.dist/pumba_{{.OS}}_{{.Arch}}
}

gox_build
