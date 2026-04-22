AI_BIN := $(HOME)/ai/bin
VERSION = $(shell cat VERSION)
LDFLAGS = -s -w -X main.version=$(VERSION)

bump:
	@awk -F. '{print $$1"."$$2"."$$3+1}' VERSION > VERSION.tmp && mv VERSION.tmp VERSION
	@echo "Version: $$(cat VERSION)"

build: clean build-linux build-linux-nogui build-windows

build-linux: bump
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/bin/skrynia ./cmd/skrynia

build-linux-nogui: bump
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags nogui -ldflags "$(LDFLAGS)" -o build/bin/skrynia-cli ./cmd/skrynia

build-windows: bump
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o build/bin/skrynia.exe ./cmd/skrynia

install: build
	mkdir -p $(AI_BIN)
	cp build/bin/skrynia $(AI_BIN)/skrynia
	cp build/bin/skrynia-cli $(AI_BIN)/skrynia-cli
	cp build/bin/skrynia.exe $(AI_BIN)/skrynia.exe
	chmod +x $(AI_BIN)/skrynia $(AI_BIN)/skrynia-cli

test:
	sg tss -c "go test ./..."

clean:
	rm -rf build/bin/*
