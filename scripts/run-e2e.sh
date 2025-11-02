#!/bin/bash

# 设置环境变量
# export VOLCENGINE_AK="your-ak"
# export VOLCENGINE_SK="your-sk"
# export VOLCENGINE_REGION="cn-beijing"
# export VOLCENGINE_CLUSTER_ID="your-cluster-id"
# export TEST_DOMAIN_NAME="test.example.com"
# export PRIVATE_ZONE_ID="123456"

# 运行E2E测试
cd $(dirname $0)/..
#go test -v ./e2e/...
go test -v ./e2e/... -ginkgo.v -ginkgo.trace -ginkgo.show-node-events -test.v --timeout=30m
