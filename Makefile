IMAGE_NAME?=external-dns-volcengine-webhook
IMAGE_TAG?=latest
DOCKER?=docker

all:
	go build -o build/external-dns-volcengine-webhook ./main.go

clean:
	rm -f ./build/external-dns-volcengine-webhook

image-local:
	$(DOCKER) build -t $(IMAGE_NAME):$(IMAGE_TAG) --platform linux/amd64 -f Dockerfile .

test: 
	go test ./pkg/volcengine -v