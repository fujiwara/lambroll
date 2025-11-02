.PHONY: test binary install clean dist
cmd/lambroll/lambroll: *.go cmd/lambroll/*.go go.mod go.sum
	cd cmd/lambroll && go build -ldflags "-s -w" -gcflags="-trimpath=${PWD}"

install: cmd/lambroll/lambroll
	install cmd/lambroll/lambroll ${GOPATH}/bin

test:
	go test -race ./...

clean:
	rm -f cmd/lambroll/lambroll
	rm -fr dist/

packages:
	goreleaser build --skip-validate --clean

packages-snapshot:
	goreleaser build --skip-validate --clean --snapshot

orb/publish:
	circleci orb validate circleci-orb.yml
	circleci orb publish circleci-orb.yml $(ORB_NAMESPACE)/lambroll@dev:latest

orb/promote:
	circleci orb publish promote $(ORB_NAMESPACE)/lambroll@dev:latest patch
