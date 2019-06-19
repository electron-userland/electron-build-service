.PHONY: router builder docker json apply bundle

builder:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/builder ./cmd/builder

# brew install golangci/tap/golangci-lint && brew upgrade golangci/tap/golangci-lint
lint:
	golangci-lint run

docker: builder
	docker build -f cmd/builder/Dockerfile -t electronuserland/build-service-builder .

push-docker: docker
	docker push electronuserland/build-service-builder

# don't forget to do push-docker
bundle:
	./scripts/build-bundle.sh

dev: docker
	DEBUG=electron-builder SNAP_DESTRUCTIVE_MODE=true USE_EMBEDDED_ETCD=true BUILDER_HOST=0.0.0.0 docker-compose up --abort-on-container-exit --remove-orphans --renew-anon-volumes

mp: builder
	multipass launch --name build-service || true

	multipass umount build-service:/project || true
	multipass mount . build-service:/project

	multipass exec build-service /project/scripts/prepare-linux-host.sh
	multipass exec build-service start.sh

# https://github.com/rancher/cli/releases
apply: bundle
	rancher kubectl apply -f k8s/builder.yaml

add-cluster-resources: bundle
	rancher kubectl apply -f k8s/logspout.yaml
	rancher kubectl apply -f k8s/builder.yaml
	rancher kubectl apply -f k8s/builder-service.yaml

update-deps:
	go get -u ./cmd/builder
	go mod tidy

# rsync -r ~/Documents/electron-builder/packages/app-builder-lib/out/ ~/Documents/electron-build-service/node_modules/app-builder-lib/out