FROM electronuserland/builder:latest

COPY node_modules /node_modules

RUN ln -s /node_modules/7zip-bin/linux/x64/7za /usr/local/bin/7za && /node_modules/app-builder-bin/linux/x64/app-builder prefetch-tools

COPY out/linux/builder /builder

CMD ["/builder"]