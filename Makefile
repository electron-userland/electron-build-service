.PHONY: router builder docker json apply bundle

all: router builder

router:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/router ./cmd/router

builder:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/builder ./cmd/builder

# brew install golangci/tap/golangci-lint && brew upgrade golangci/tap/golangci-lint
lint:
	golangci-lint run

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
	docker-compose pull etcd-cluster-client
	BUILDER_HOST=localhost BUILDER_PORT=8444 docker-compose up --abort-on-container-exit --remove-orphans --renew-anon-volumes

# https://github.com/rancher/cli/releases
apply: bundle
	rancher kubectl apply -f k8s/builder.yml
	rancher kubectl apply -f k8s/router.yml