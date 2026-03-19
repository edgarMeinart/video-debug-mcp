FROM alpine:3.19
RUN apk add --no-cache curl && \
    ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
      curl -L -o /usr/local/bin/packager "https://github.com/shaka-project/shaka-packager/releases/latest/download/packager-linux-x64"; \
    elif [ "$ARCH" = "aarch64" ]; then \
      curl -L -o /usr/local/bin/packager "https://github.com/shaka-project/shaka-packager/releases/latest/download/packager-linux-arm64"; \
    fi && \
    chmod +x /usr/local/bin/packager && \
    apk del curl
WORKDIR /workspace
ENTRYPOINT []
