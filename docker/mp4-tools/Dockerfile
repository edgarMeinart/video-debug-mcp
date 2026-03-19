FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends \
      gpac \
      curl \
      unzip \
      ca-certificates \
    && ARCH=$(uname -m) \
    && if [ "$ARCH" = "x86_64" ]; then \
         curl -L -o /tmp/bento4.zip "https://www.bok.net/Bento4/binaries/Bento4-SDK-1-6-0-641.x86_64-unknown-linux.zip"; \
       elif [ "$ARCH" = "aarch64" ]; then \
         curl -L -o /tmp/bento4.zip "https://www.bok.net/Bento4/binaries/Bento4-SDK-1-6-0-641.aarch64-unknown-linux.zip"; \
       fi \
    && cd /tmp && unzip bento4.zip \
    && cp Bento4-SDK-*/bin/mp4dump /usr/local/bin/ \
    && rm -rf /tmp/bento4* \
    && apt-get purge -y curl unzip \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /workspace
ENTRYPOINT []
