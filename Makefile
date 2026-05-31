.PHONY: test binary install clean dist

# tfstate-lookup build tags: exclude GCS and AzureRM remote backends.
# lambroll only supports the S3 and Terraform Cloud (remote) backends.
GO_BUILD_TAGS := no_gcs,no_azurerm

cmd/lambroll/lambroll: *.go cmd/lambroll/*.go go.mod go.sum
	cd cmd/lambroll && go build -tags "$(GO_BUILD_TAGS)" -ldflags "-s -w" -gcflags="-trimpath=${PWD}"

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
