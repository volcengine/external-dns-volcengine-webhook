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

set -euo pipefail

# Usage:
#   ./scripts/run-e2e.sh                 # standard e2e, excludes cross-account CLI case
#   ./scripts/run-e2e.sh standard
#   ./scripts/run-e2e.sh cross-account   # run only cross-account CLI case
#   ./scripts/run-e2e.sh all             # run all e2e cases
#
# Common env:
# export VOLCENGINE_REGION="cn-beijing"
# export PRIVATE_ZONE_ID="123456"
#
# Standard e2e env:
# export VOLCENGINE_AK="your-ak"
# export VOLCENGINE_SK="your-sk"
# export VOLCENGINE_CLUSTER_ID="your-cluster-id"
# export TEST_DOMAIN_NAME="test.example.com"
# export EXTERNAL_DNS_POLICY="sync"   # optional: sync | upsert-only, default sync
#
# Cross-account e2e env:
# export VOLCENGINE_AK="your-source-ak"
# export VOLCENGINE_SK="your-source-sk"
# export VOLCENGINE_ROLE_TRN="trn:iam::200000000000:role/external-dns-target"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
MODE="${1:-standard}"

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required environment variable: ${name}" >&2
    exit 1
  fi
}

require_source_credentials() {
  if [[ -n "${VOLCENGINE_AK:-}" && -n "${VOLCENGINE_SK:-}" ]]; then
    return
  fi
  if [[ -n "${VOLCENGINE_OIDC_TOKEN_FILE:-}" && -n "${VOLCENGINE_OIDC_ROLE_TRN:-}" ]]; then
    return
  fi
  echo "missing source credentials: set VOLCENGINE_AK/VOLCENGINE_SK or VOLCENGINE_OIDC_TOKEN_FILE/VOLCENGINE_OIDC_ROLE_TRN" >&2
  exit 1
}

run_go_test() {
  local label_filter="$1"
  cd "${REPO_ROOT}"
  if [[ -n "${label_filter}" ]]; then
    go test -v ./e2e/... -ginkgo.v -ginkgo.trace -ginkgo.show-node-events -ginkgo.label-filter="${label_filter}" -test.v --timeout=30m
    return
  fi
  go test -v ./e2e/... -ginkgo.v -ginkgo.trace -ginkgo.show-node-events -test.v --timeout=30m
}

case "${MODE}" in
  standard)
    require_source_credentials
    require_env "VOLCENGINE_REGION"
    require_env "PRIVATE_ZONE_ID"
    require_env "TEST_DOMAIN_NAME"
    if [[ -z "${VOLCENGINE_CLUSTER_ID:-}" && -z "${VOLCENGINE_CLUSTER_NAME:-}" ]]; then
      echo "missing cluster identity: set VOLCENGINE_CLUSTER_ID or VOLCENGINE_CLUSTER_NAME" >&2
      exit 1
    fi
    run_go_test "!cross-account"
    ;;
  cross-account)
    require_source_credentials
    require_env "VOLCENGINE_REGION"
    require_env "PRIVATE_ZONE_ID"
    require_env "VOLCENGINE_ROLE_TRN"
    require_env "CROSS_ACCOUNT_TEST_RECORD_HOST"
    export VOLCENGINE_E2E_CROSS_ACCOUNT="true"
    run_go_test "cross-account"
    ;;
  all)
    require_source_credentials
    require_env "VOLCENGINE_REGION"
    require_env "PRIVATE_ZONE_ID"
    require_env "TEST_DOMAIN_NAME"
    if [[ -z "${VOLCENGINE_CLUSTER_ID:-}" && -z "${VOLCENGINE_CLUSTER_NAME:-}" ]]; then
      echo "missing cluster identity: set VOLCENGINE_CLUSTER_ID or VOLCENGINE_CLUSTER_NAME" >&2
      exit 1
    fi
    if [[ -n "${VOLCENGINE_ROLE_TRN:-}" ]]; then
      require_env "CROSS_ACCOUNT_TEST_RECORD_HOST"
      export VOLCENGINE_E2E_CROSS_ACCOUNT="true"
    fi
    run_go_test ""
    ;;
  *)
    echo "unsupported mode: ${MODE}. expected one of: standard, cross-account, all" >&2
    exit 1
    ;;
esac
