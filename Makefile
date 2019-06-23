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
	./scripts/set-image-digest.sh
	make create-self-hosted

create-self-hosted:
	# https://github.com/kubernetes-sigs/kustomize/issues/766
	ln -f certs/tls.cert k8s/overlays/single-node/tls.cert
	ln -f certs/tls.key k8s/overlays/single-node/tls.key
	kustomize build k8s/overlays/single-node --output k8s/generated/self-hosted.yaml

dev: docker
	DEBUG=electron-builder SNAP_DESTRUCTIVE_MODE=true USE_EMBEDDED_ETCD=true BUILDER_HOST=0.0.0.0 docker-compose up --abort-on-container-exit --remove-orphans --renew-anon-volumes

mp-local-cluster: builder
	multipass launch --name build-service --cpus 4 18.04 || true

	multipass umount build-service:/project || true
	multipass mount . build-service:/project

	multipass exec build-service /project/scripts/install-local-k8s.sh

# https://github.com/rancher/cli/releases
apply: bundle
	rancher kubectl apply -f k8s/builder.yaml

add-cluster-resources: bundle
	# to see full effective definition: kustomize build k8s/overlays/production --output k8s/generated/production.yaml
	rancher kubectl apply -k k8s/overlays/production

update-deps:
	go get -u ./cmd/builder
	go mod tidy

# rsync -r ~/Documents/electron-builder/packages/app-builder-lib/out/ ~/Documents/electron-build-service/node_modules/app-builder-lib/out