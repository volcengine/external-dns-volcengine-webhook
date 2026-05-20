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

package server

import (
	"testing"

	"volcengine-provider/pkg/volcengine"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildProviderOptionsWithoutAssumeRoleKeepsCompatibility(t *testing.T) {
	v := viper.New()
	v.Set("port", 8888)
	v.Set("access_key", "source-ak")
	v.Set("secret_key", "source-sk")
	v.Set("vpc", "vpc-123")
	v.Set("region", "cn-beijing")
	v.Set("privatezone_endpoint", "privatezone.example.com")
	v.Set("domain_filter", "example.com,internal.example.com")

	options, err := buildProviderOptions(loadStartConfig(v))
	require.NoError(t, err)

	cfg := applyOptions(options)
	require.NotNil(t, cfg.Credentials)
	assert.Nil(t, cfg.AssumeRole)
	assert.Equal(t, "cn-beijing", cfg.RegionID)
	assert.Equal(t, "vpc-123", cfg.VpcId)
	assert.Equal(t, "privatezone.example.com", cfg.PrivateZoneEndpoint)
	assert.Equal(t, []string{"example.com", "internal.example.com"}, cfg.DomainFilter)

	value, err := cfg.Credentials.Get()
	require.NoError(t, err)
	assert.Equal(t, "source-ak", value.AccessKeyID)
	assert.Equal(t, "source-sk", value.SecretAccessKey)
}

func TestBuildProviderOptionsWithStaticCredentialsAndAssumeRole(t *testing.T) {
	v := viper.New()
	v.Set("access_key", "source-ak")
	v.Set("secret_key", "source-sk")
	v.Set("vpc", "vpc-123")
	v.Set("region", "cn-beijing")
	v.Set("privatezone_endpoint", "privatezone.example.com")
	v.Set("sts_endpoint", "sts.example.com")
	v.Set("role_trn", "trn:iam::123456789012:role/target")
	v.Set("role_session_name", "custom-session")
	v.Set("duration_seconds", 3600)

	options, err := buildProviderOptions(loadStartConfig(v))
	require.NoError(t, err)

	cfg := applyOptions(options)
	require.NotNil(t, cfg.Credentials)
	require.NotNil(t, cfg.AssumeRole)
	assert.Equal(t, "sts.example.com", cfg.AssumeRole.STSEndpoint)
	assert.Equal(t, "trn:iam::123456789012:role/target", cfg.AssumeRole.RoleTrn)
	assert.Equal(t, "custom-session", cfg.AssumeRole.RoleSessionName)
	assert.Equal(t, int32(3600), cfg.AssumeRole.DurationSeconds)

	value, err := cfg.Credentials.Get()
	require.NoError(t, err)
	assert.Equal(t, "source-ak", value.AccessKeyID)
	assert.Equal(t, "source-sk", value.SecretAccessKey)
}

func TestBuildProviderOptionsWithOIDCSourceAndAssumeRole(t *testing.T) {
	v := viper.New()
	v.Set("oidc_token_file", "/var/run/secrets/tokens/oidc-token")
	v.Set("oidc_role_trn", "trn:iam::111111111111:role/source")
	v.Set("vpc", "vpc-123")
	v.Set("region", "cn-beijing")
	v.Set("privatezone_endpoint", "privatezone.example.com")
	v.Set("sts_endpoint", "sts.example.com")
	v.Set("role_trn", "trn:iam::222222222222:role/target")
	v.Set("duration_seconds", 1800)

	options, err := buildProviderOptions(loadStartConfig(v))
	require.NoError(t, err)

	cfg := applyOptions(options)
	require.NotNil(t, cfg.Credentials)
	require.NotNil(t, cfg.AssumeRole)
	assert.Equal(t, "sts.example.com", cfg.AssumeRole.STSEndpoint)
	assert.Equal(t, "trn:iam::222222222222:role/target", cfg.AssumeRole.RoleTrn)
	assert.Equal(t, int32(1800), cfg.AssumeRole.DurationSeconds)
}

func TestBuildProviderOptionsRequiresSourceCredentials(t *testing.T) {
	_, err := buildProviderOptions(startConfig{})
	require.EqualError(t, err, "aksk or oidc token file is required")
}

func applyOptions(options []volcengine.Option) *volcengine.Config {
	cfg := &volcengine.Config{}
	for _, option := range options {
		option(cfg)
	}
	return cfg
}
