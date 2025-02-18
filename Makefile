VOLCENGINE_ACCESS_SECRET?=1234
VOLCENGINE_ACCESS_KEY?=1234
VOLCENGINE_VPC?=vpc-13f39admd7r403n6nu5q6s0i8
VOLCENGINE_ENDPOINT?="open.volcengineapi.com"
VOLCENGINE_REGION?=cn-beijing

IMAGE_NAME?=external-dns-volcengine-webhook
IMAGE_TAG?=latest
DOCKER?=sudo nerdctl
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
	$(DOCKER) build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile .

run-image-local:
	$(DOCKER) run --rm -it -p 8888:8888 \
		-e VOLCENGINE_ACCESS_SECRET=$(VOLCENGINE_ACCESS_SECRET) \
		-e VOLCENGINE_ACCESS_KEY=$(VOLCENGINE_ACCESS_KEY) \
		-e VOLCENGINE_VPC=$(VOLCENGINE_VPC) \
		-e VOLCENGINE_REGION=$(VOLCENGINE_REGION) \
		-e VOLCENGINE_ENDPOINT=$(VOLCENGINE_ENDPOINT) \
		$(IMAGE_NAME):$(IMAGE_TAG) \
		/opt/external-dns-volcengine-webhook start --port=8888 --debug

# 添加 helm 安装命令
helm-install:
	helm upgrade --install external-dns \
		manifests/externaldns \
		--namespace kube-system \
		--set userConfig.env.ak=${VOLCENGINE_ACCESS_KEY} \
		--set userConfig.env.sk=${VOLCENGINE_ACCESS_SECRET} \
		--set userConfig.env.vpc=${VOLCENGINE_VPC} \
		--set userConfig.env.region=${VOLCENGINE_REGION} \
		--set userConfig.env.endpoint=${VOLCENGINE_ENDPOINT} \
		--set userConfig.args.controller.domainFilter=${TARGET_DOMAIN} \
		--set userConfig.args.provider.enableDebug=true \
		--set publicConfig.image.controller.repository=cr-helm-test-cn-beijing.cr.volces.com/test/external-dns \
		--set publicConfig.image.provider.repository=registry.cn-hangzhou.aliyuncs.com/wzkpublic/external-dns-volcengine-webhook \
		--debug

helm-template:
	helm template external-dns manifests/externaldns \
		--namespace kube-system \
		--set userConfig.env.ak=${VOLCENGINE_ACCESS_KEY} \
		--set userConfig.env.sk=${VOLCENGINE_ACCESS_SECRET} \
		--set userConfig.env.vpc=${VOLCENGINE_VPC} \
		--set userConfig.env.region=${VOLCENGINE_REGION} \
		--set userConfig.env.endpoint=${VOLCENGINE_ENDPOINT} \
		--set userConfig.args.controller.domainFilter=${TARGET_DOMAIN} \
		--set userConfig.args.provider.enableDebug=true \
		--debug

helm-uninstall:
	helm uninstall external-dns -n kube-system
