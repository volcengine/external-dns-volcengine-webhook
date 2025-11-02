#!/bin/bash

# Copyright 2025 The Beijing Volcano Engine Technology Co., Ltd. Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# Env
# export VOLCENGINE_AK="your-ak"
# export VOLCENGINE_SK="your-sk"
# export VOLCENGINE_REGION="cn-beijing"
# export VOLCENGINE_CLUSTER_ID="your-cluster-id"
# export TEST_DOMAIN_NAME="test.example.com"
# export PRIVATE_ZONE_ID="123456"

cd $(dirname $0)/..
go test -v ./e2e/... -ginkgo.v -ginkgo.trace -ginkgo.show-node-events -test.v --timeout=30m
