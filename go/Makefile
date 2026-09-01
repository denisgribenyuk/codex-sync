APP := codex-sync
DIST := dist

.PHONY: build clean \
        build-all \
        linux-amd64 linux-arm64 \
        darwin-amd64 darwin-arm64 \
        windows-amd64 windows-arm64

build:
	go build -trimpath -ldflags="-s -w" -o $(APP) .

clean:
	rm -rf $(DIST) $(APP)

build-all: clean \
	linux-amd64 \
	linux-arm64 \
	darwin-amd64 \
	darwin-arm64 \
	windows-amd64 \
	windows-arm64

linux-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-linux-amd64 .

linux-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-linux-arm64 .

darwin-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-darwin-amd64 .

darwin-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-darwin-arm64 .

windows-amd64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-windows-amd64.exe .

windows-arm64:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		go build -trimpath -ldflags="-s -w" \
		-o $(DIST)/$(APP)-windows-arm64.exe .