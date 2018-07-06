GOPATH = $(CURDIR)/vendor

.PHONY: router builder docker json

all: router builder

router:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/router ./cmd/router

builder:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/builder ./cmd/builder

# vetshadow is disabled because of err (https://groups.google.com/forum/#!topic/golang-nuts/ObtoxsN7AWg)
lint:
	gometalinter --aggregate --sort=path --vendor --skip=node_modules ./...

docker: all
	docker build -f cmd/builder/Dockerfile -t electronuserland/build-service-builder .
	docker build -f cmd/router/Dockerfile -t electronuserland/build-service-router .

push-docker: docker
	docker push electronuserland/build-service-builder
	docker push electronuserland/build-service-router

# don't forget to do push-docker
bundle:
	./scripts/build-bundle.sh

dev: docker
	BUILDER_HOST=localhost BUILDER_PORT=8444 docker-compose up --abort-on-container-exit --remove-orphans --renew-anon-volumes