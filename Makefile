GIT_VER := $(shell git describe --tags)
DATE := $(shell date +%Y-%m-%dT%H:%M:%S%z)
export GO111MODULE := on

.PHONY: test binary install clean
cmd/lambroll/lambroll: *.go cmd/lambroll/*.go
	cd cmd/lambroll && go build -ldflags "-s -w -X main.Version=${GIT_VER} -X main.buildDate=${DATE}" -gcflags="-trimpath=${PWD}"

install: cmd/lambroll/lambroll
	install cmd/lambroll/lambroll ${GOPATH}/bin

test:
	go test -race .
	go test -race ./cmd/lambroll

clean:
	rm -f cmd/lambroll/lambroll
	rm -f pkg/*

release:
	ghr -u fujiwara -r lambroll -n "$(GIT_VER)" $(GIT_VER) dist/
