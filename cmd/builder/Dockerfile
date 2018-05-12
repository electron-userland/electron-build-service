FROM electronuserland/builder:latest

RUN curl -L https://github.com/electron-userland/electron-builder-binaries/releases/download/zstd-linux-1.3.4/zstd --output /usr/local/bin/zstd && chmod +x /usr/local/bin/zstd && \
	curl -SL https://registry.npmjs.org/app-builder-bin-linux/-/app-builder-bin-linux-1.8.9.tgz | tar xzv --strip=2 package/x64/app-builder && mv app-builder /usr/local/bin/app-builder && \
	curl -SL https://registry.npmjs.org/7zip-bin-linux/-/7zip-bin-linux-1.3.1.tgz | tar xzv --strip=2 package/x64/7za && mv 7za /usr/local/bin/7za && \
	/usr/local/bin/app-builder prefetch-tools

COPY node_modules /node_modules
COPY out/linux/builder /builder

CMD ["/builder"]