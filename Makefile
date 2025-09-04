VOLCENGINE_ACCESS_SECRET?=1234
VOLCENGINE_ACCESS_KEY?=1234
VOLCENGINE_VPC?=vpc-13f39admd7r403n6nu5q6s0i8
VOLCENGINE_ENDPOINT?="open.volcengineapi.com"
VOLCENGINE_REGION?=cn-beijing

IMAGE_NAME?=external-dns-volcengine-webhook
IMAGE_TAG?=latest
DOCKER?=docker
TARGET_DOMAIN?=test.com

all:
	go build -o build/external-dns-volcengine-webhook ./main.go

clean:
	rm -f ./build/external-dns-volcengine-webhook

run: all
	VOLCENGINE_ACCESS_SECRET=$(VOLCENGINE_ACCESS_SECRET) \
	VOLCENGINE_ACCESS_KEY=$(VOLCENGINE_ACCESS_KEY) \
	VOLCENGINE_VPC=$(VOLCENGINE_VPC) \
	VOLCENGINE_REGION=$(VOLCENGINE_REGION) \
	VOLCENGINE_ENDPOINT=$(VOLCENGINE_ENDPOINT) \
	./build/external-dns-volcengine-webhook start --port=8888 --debug

image-local:
	$(DOCKER) build -t $(IMAGE_NAME):$(IMAGE_TAG) --platform linux/amd64 -f Dockerfile .

run-image-local:
	$(DOCKER) run --rm -it -p 8888:8888 \
		-e VOLCENGINE_ACCESS_SECRET=$(VOLCENGINE_ACCESS_SECRET) \
		-e VOLCENGINE_ACCESS_KEY=$(VOLCENGINE_ACCESS_KEY) \
		-e VOLCENGINE_VPC=$(VOLCENGINE_VPC) \
		-e VOLCENGINE_REGION=$(VOLCENGINE_REGION) \
		-e VOLCENGINE_ENDPOINT=$(VOLCENGINE_ENDPOINT) \
		$(IMAGE_NAME):$(IMAGE_TAG) \
		/opt/external-dns-volcengine-webhook start --port=8888 --debug

