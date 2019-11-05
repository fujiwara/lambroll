GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: test binary install clean dist
cmd/lambroll/lambroll: *.go cmd/lambroll/*.go
	cd cmd/lambroll && go build -ldflags "-s -w -X main.Version=${GIT_VER}" -gcflags="-trimpath=${PWD}"

install: cmd/lambroll/lambroll
	install cmd/lambroll/lambroll ${GOPATH}/bin

test:
	go test -race .
	go test -race ./cmd/lambroll

clean:
	rm -f cmd/lambroll/lambroll
	rm -fr dist/

dist:
	CGO_ENABLED=0 \
		goxz -pv=$(GIT_VER) \
		-build-ldflags="-s -w -X main.Version=${GIT_VER}" \
		-os=darwin,linux -arch=amd64 -d=dist ./cmd/lambroll

release:
	ghr -u fujiwara -r lambroll -n "$(GIT_VER)" $(GIT_VER) dist/
