.PHONY: router builder docker json apply bundle

builder:
	GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o out/linux/builder ./cmd/builder

# brew install golangci/tap/golangci-lint && brew upgrade golangci/tap/golangci-lint
lint:
	golangci-lint run

docker:
	docker build -f cmd/builder/Dockerfile -t electronuserland/build-service-builder .

push-docker: docker
	docker push electronuserland/build-service-builder:latest

bundle: push-docker
	./scripts/set-image-digest.sh
	make create-self-hosted

create-self-hosted:
	kustomize build k8s/overlays/single-node --output k8s/generated/self-hosted.yaml

dev:
	tilt up

mp-local-cluster: builder
	multipass launch --name build-service --cpus 4 18.04 || true

	multipass umount build-service:/project || true
	multipass mount . build-service:/project

	multipass exec build-service /project/scripts/install-local-k8s.sh

# https://github.com/rancher/cli/releases
add-cluster-resources: bundle
	# to switch context if needed (https://rancher.com/docs/rancher/v2.x/en/cli/#project-selection): rancher context switch
	# to see full effective definition: kustomize build k8s/overlays/production --output k8s/generated/production.yaml
	rancher kubectl apply -k k8s/overlays/production

update-deps:
	GOPROXY=https://proxy.golang.org go get -u ./cmd/builder
	go mod tidy

# rsync -r ~/Documents/electron-builder/packages/app-builder-lib/out/ ~/Documents/electron-build-service/node_modules/app-builder-lib/out