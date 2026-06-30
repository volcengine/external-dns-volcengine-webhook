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
	"fmt"
	"os"
	"strconv"
	"strings"

	provider "volcengine-provider/pkg/volcengine"

	"github.com/volcengine/volcengine-go-sdk/service/vke"
	sdkvolcengine "github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// TestConfig stores configuration information needed for testing
type TestConfig struct {
	AK                  string
	SK                  string
	RegionID            string
	ClusterID           string
	ClusterName         string
	DomainName          string
	PrivateZoneID       string
	STSEndpoint         string
	OIDCTokenFile       string
	OIDCRoleTrn         string
	RoleTrn             string
	RoleSessionName     string
	DurationSeconds     int32
	CrossAccountEnabled bool
	CrossAccountHost    string
	CrossAccountType    string
	CrossAccountValue   string
	ExternalDNSPolicy   string
}

// LoadTestConfig loads test configuration from environment variables or config file
func LoadTestConfig() (*TestConfig, error) {
	config, err := loadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if !config.HasSourceCredentials() || (config.ClusterID == "" && config.ClusterName == "") {
		return nil, fmt.Errorf("source credentials and either VOLCENGINE_CLUSTER_ID or VOLCENGINE_CLUSTER_NAME environment variables must be provided")
	}

	return config, nil
}

func LoadCrossAccountTestConfig() (*TestConfig, error) {
	config, err := loadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if !config.CrossAccountEnabled {
		return config, nil
	}
	if !config.HasSourceCredentials() {
		return nil, fmt.Errorf("source credentials must be provided for cross-account e2e")
	}
	if config.PrivateZoneID == "" {
		return nil, fmt.Errorf("PRIVATE_ZONE_ID environment variable must be provided for cross-account e2e")
	}
	if config.RoleTrn == "" {
		return nil, fmt.Errorf("VOLCENGINE_ROLE_TRN environment variable must be provided for cross-account e2e")
	}
	if config.CrossAccountHost == "" {
		return nil, fmt.Errorf("CROSS_ACCOUNT_TEST_RECORD_HOST environment variable must be provided for cross-account e2e")
	}

	return config, nil
}

func loadConfigFromEnv() (*TestConfig, error) {
	config := &TestConfig{
		AK:                  os.Getenv("VOLCENGINE_AK"),
		SK:                  os.Getenv("VOLCENGINE_SK"),
		RegionID:            os.Getenv("VOLCENGINE_REGION"),
		ClusterID:           os.Getenv("VOLCENGINE_CLUSTER_ID"),
		ClusterName:         os.Getenv("VOLCENGINE_CLUSTER_NAME"),
		DomainName:          os.Getenv("TEST_DOMAIN_NAME"),
		PrivateZoneID:       os.Getenv("PRIVATE_ZONE_ID"),
		STSEndpoint:         os.Getenv("VOLCENGINE_STS_ENDPOINT"),
		OIDCTokenFile:       os.Getenv("VOLCENGINE_OIDC_TOKEN_FILE"),
		OIDCRoleTrn:         os.Getenv("VOLCENGINE_OIDC_ROLE_TRN"),
		RoleTrn:             os.Getenv("VOLCENGINE_ROLE_TRN"),
		RoleSessionName:     os.Getenv("VOLCENGINE_ROLE_SESSION_NAME"),
		CrossAccountEnabled: os.Getenv("VOLCENGINE_E2E_CROSS_ACCOUNT") == "true",
		CrossAccountHost:    os.Getenv("CROSS_ACCOUNT_TEST_RECORD_HOST"),
		CrossAccountType:    os.Getenv("CROSS_ACCOUNT_TEST_RECORD_TYPE"),
		CrossAccountValue:   os.Getenv("CROSS_ACCOUNT_TEST_RECORD_VALUE"),
		ExternalDNSPolicy:   normalizeExternalDNSPolicy(os.Getenv("EXTERNAL_DNS_POLICY")),
	}

	durationSeconds, err := envInt32("VOLCENGINE_DURATION_SECONDS")
	if err != nil {
		return nil, err
	}
	config.DurationSeconds = durationSeconds

	if config.RegionID == "" {
		config.RegionID = "cn-beijing"
	}
	if config.CrossAccountType == "" {
		config.CrossAccountType = "A"
	}
	if config.CrossAccountValue == "" {
		config.CrossAccountValue = "192.0.2.10"
	}

	return config, nil
}

func normalizeExternalDNSPolicy(policy string) string {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	switch normalized {
	case "", "sync":
		return "sync"
	case "upsert-only":
		return "upsert-only"
	default:
		return normalized
	}
}

func (c *TestConfig) HasSourceCredentials() bool {
	return (c.AK != "" && c.SK != "") || (c.OIDCTokenFile != "" && c.OIDCRoleTrn != "")
}

func (c *TestConfig) SourceCredentialOptions() provider.CredentialOptions {
	return provider.CredentialOptions{
		AccessKey:     c.AK,
		SecretKey:     c.SK,
		STSEndpoint:   c.STSEndpoint,
		OIDCTokenFile: c.OIDCTokenFile,
		OIDCRoleTrn:   c.OIDCRoleTrn,
	}
}

func (c *TestConfig) TargetCredentialOptions() provider.CredentialOptions {
	options := c.SourceCredentialOptions()
	options.AssumeRoleConfig = &provider.AssumeRoleOptions{
		Region:          c.RegionID,
		STSEndpoint:     c.STSEndpoint,
		RoleTrn:         c.RoleTrn,
		RoleSessionName: c.RoleSessionName,
		DurationSeconds: c.DurationSeconds,
	}
	return options
}

// CreateSourceVolcengineClient creates a source-credential Volcengine client.
func CreateSourceVolcengineClient(config *TestConfig) (*sdkvolcengine.Config, error) {
	creds, err := provider.NewCredentials(config.SourceCredentialOptions())
	if err != nil {
		return nil, err
	}
	return sdkvolcengine.NewConfig().
		WithCredentials(creds).
		WithRegion(config.RegionID), nil
}

// CreateTargetVolcengineClient creates a target-account Volcengine client.
func CreateTargetVolcengineClient(config *TestConfig) (*sdkvolcengine.Config, error) {
	creds, err := provider.NewCredentials(config.TargetCredentialOptions())
	if err != nil {
		return nil, err
	}
	return sdkvolcengine.NewConfig().
		WithCredentials(creds).
		WithRegion(config.RegionID), nil
}

// CreateVolcengineClient creates a Volcengine client.
func CreateVolcengineClient(config *TestConfig) (*sdkvolcengine.Config, error) {
	return CreateTargetVolcengineClient(config)
}

func envInt32(name string) (int32, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", name, value, err)
	}
	return int32(parsed), nil
}

// GetClusterKubeconfig gets the public kubeconfig of a cluster through OpenAPI
func GetClusterKubeconfig(config *TestConfig) (string, error) {
	// First create Volcengine configuration
	volcConfig, err := CreateSourceVolcengineClient(config)
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
		resp, listErr := vkeClient.ListClusters(listClustersInput)
		if listErr != nil {
			return "", fmt.Errorf("failed to list clusters: %w", listErr)
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
			ClusterIds: sdkvolcengine.StringSlice([]string{config.ClusterID}),
			Types:      sdkvolcengine.StringSlice([]string{"Public"}),
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

	// Return the kubeconfig configuration string directly.
	return *kubeconfig.Kubeconfig, nil
}
