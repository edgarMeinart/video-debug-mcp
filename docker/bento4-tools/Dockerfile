FROM alpine:3.19
RUN apk add --no-cache curl unzip && \
    ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
      curl -L -o /tmp/bento4.zip "https://www.bok.net/Bento4/binaries/Bento4-SDK-1-6-0-641.x86_64-unknown-linux.zip"; \
    elif [ "$ARCH" = "aarch64" ]; then \
      curl -L -o /tmp/bento4.zip "https://www.bok.net/Bento4/binaries/Bento4-SDK-1-6-0-641.aarch64-unknown-linux.zip"; \
    fi && \
    cd /tmp && unzip bento4.zip && \
    cp Bento4-SDK-*/bin/* /usr/local/bin/ && \
    rm -rf /tmp/bento4* && \
    apk del curl unzip
WORKDIR /workspace
ENTRYPOINT []
