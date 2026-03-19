.PHONY: build test test-integration test-all build-images clean

build:
	go build -o bin/video-debug-mcp ./cmd/server

test:
	go test ./...

test-integration:
	go test -tags integration ./...

test-all: test test-integration

build-images:
	docker build -t video-debug/ffmpeg-tools ./docker/ffmpeg-tools
	docker build -t video-debug/mp4-tools ./docker/mp4-tools
	docker build -t video-debug/bento4-tools ./docker/bento4-tools
	docker build -t video-debug/mediainfo-tools ./docker/mediainfo-tools
	docker build -t video-debug/shaka-tools ./docker/shaka-tools

clean:
	rm -rf bin/
