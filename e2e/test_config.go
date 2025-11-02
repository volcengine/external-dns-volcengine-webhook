// Copyright 2025 The Beijing Volcano Engine Technology Co., Ltd. Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package e2e

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/vke"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// TestConfig stores configuration information needed for testing
type TestConfig struct {
	AK            string
	SK            string
	RegionID      string
	ClusterID     string
	ClusterName   string
	DomainName    string
	PrivateZoneID string
}

// LoadTestConfig loads test configuration from environment variables or config file
func LoadTestConfig() (*TestConfig, error) {
	config := &TestConfig{
		AK:            os.Getenv("VOLCENGINE_AK"),
		SK:            os.Getenv("VOLCENGINE_SK"),
		RegionID:      os.Getenv("VOLCENGINE_REGION"),
		ClusterID:     os.Getenv("VOLCENGINE_CLUSTER_ID"),
		ClusterName:   os.Getenv("VOLCENGINE_CLUSTER_NAME"),
		DomainName:    os.Getenv("TEST_DOMAIN_NAME"),
		PrivateZoneID: os.Getenv("PRIVATE_ZONE_ID"),
	}

	if config.AK == "" || config.SK == "" || (config.ClusterID == "" && config.ClusterName == "") {
		return nil, fmt.Errorf("VOLCENGINE_AK, VOLCENGINE_SK, and either VOLCENGINE_CLUSTER_ID or VOLCENGINE_CLUSTER_NAME environment variables must be provided")
	}

	if config.RegionID == "" {
		config.RegionID = "cn-beijing"
	}

	return config, nil
}

// CreateVolcengineClient creates a Volcengine client
func CreateVolcengineClient(config *TestConfig) (*volcengine.Config, error) {
	return volcengine.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(config.AK, config.SK, "")).
		WithRegion(config.RegionID), nil
}

// GetClusterKubeconfig gets the public kubeconfig of a cluster through OpenAPI
func GetClusterKubeconfig(config *TestConfig) (string, error) {
	// First create Volcengine configuration
	volcConfig, err := CreateVolcengineClient(config)
	if err != nil {
		return "", fmt.Errorf("failed to create volcengine config: %w", err)
	}

	// 使用配置创建会话
	sess, err := session.NewSession(volcConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create volcengine session: %w", err)
	}

	// 创建VKE服务客户端
	vkeClient := vke.New(sess)

	// 如果没有提供ClusterID但提供了ClusterName，需要先通过ClusterName获取ClusterID
	if config.ClusterID == "" && config.ClusterName != "" {
		// 构建ListClusters请求
		listClustersInput := &vke.ListClustersInput{}

		// 发送请求
		resp, err := vkeClient.ListClusters(listClustersInput)
		if err != nil {
			return "", fmt.Errorf("failed to list clusters: %w", err)
		}

		// 查找匹配的集群
		found := false
		for _, cluster := range resp.Items {
			if cluster.Name != nil && *cluster.Name == config.ClusterName {
				config.ClusterID = *cluster.Id
				found = true
				break
			}
		}

		if !found {
			return "", fmt.Errorf("cluster with name %s not found", config.ClusterName)
		}
	}

	// 使用ListKubeconfigs方法直接获取kubeconfig
	listKubeconfigsInput := &vke.ListKubeconfigsInput{
		Filter: &vke.FilterForListKubeconfigsInput{
			ClusterIds: volcengine.StringSlice([]string{config.ClusterID}),
			Types:      volcengine.StringSlice([]string{"Public"}),
		},
	}

	kubeconfigResp, err := vkeClient.ListKubeconfigs(listKubeconfigsInput)
	if err != nil {
		return "", fmt.Errorf("failed to list kubeconfigs: %w", err)
	}

	// Check if kubeconfig was found
	if len(kubeconfigResp.Items) == 0 {
		return "", fmt.Errorf("kubeconfig for cluster %s not found", config.ClusterID)
	}

	// Get the first kubeconfig (there should be only one matching cluster)
	kubeconfig := kubeconfigResp.Items[0]

	// Check if kubeconfig content exists
	if kubeconfig == nil || kubeconfig.Kubeconfig == nil {
		return "", fmt.Errorf("kubeconfig content is empty")
	}

	// Base64 decode the kubeconfig
	decodedKubeconfig, err := base64.RawStdEncoding.DecodeString(*kubeconfig.Kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	// Return the kubeconfig configuration string
	return string(decodedKubeconfig), nil
}
