export GO111MODULE := on
export GOPRIVATE := github.com/mlveggo/*

.PHONY: all
all: \
	go-generate \
	go-fmt \
	go-mod-tidy \
	go-vet \
	go-review \
	go-lint \
	go-test \

include tools/golangci-lint/rules.mk
include tools/goreview/rules.mk
include tools/stringer/rules.mk

.PHONY: clean
clean:
	rm -rf build

.PHONY: go-mod-tidy
go-mod-tidy:
	go mod tidy -v

.PHONY: go-fmt
go-fmt:
	go fmt ./...

.PHONY: go-vet
go-vet:
	go vet ./...

.PHONY: go-test
go-test:
	go test -race -cover ./...

.PHONY: go-generate
go-generate: \
	protocol_string.go \
	commands_string.go \
	parameters_string.go \
	packet_string.go \
	file_string.go \
	pkg/packets/timecode_string.go \
	pkg/packets/image_string.go

%_string.go: %.go $(stringer)
	$(info generating $@...)
	@go generate ./$<

.PHONY: go-build
go-build:
	mkdir build
	go build -o build ./...

.PHONY: run
run:
	go run cmd/streaming/main.go

.PHONY: run-discover
run-discover:
	go run cmd/discover/main.go

.PHONY: run-settings
run-settings:
	go run cmd/settings/main.go

