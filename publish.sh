#!/bin/sh -e

REPO="dbx-sync"
ARCHS="linux/386 linux/amd64 linux/arm darwin/amd64 darwin/386"

if [ -z "${VERSION}" ]; then
	echo "Please set \$VERSION environment variable"
	exit 1
fi

if [ -z "${GITHUB_TOKEN}" ]; then
	echo "PLease set \$GITHUB_TOKEN environment variable"
	exit 1
fi

set -x

go get github.com/aktau/github-release
go get github.com/mitchellh/gox

github-release release --user Luzifer --repo ${REPO} --tag ${VERSION} --name ${VERSION} || true

gox -ldflags="-X main.version=${VERSION}" -osarch="${ARCHS}"
for file in ${REPO}_*; do
  github-release upload --user Luzifer --repo ${REPO} --tag ${VERSION} --name ${file} --file ${file}
done
