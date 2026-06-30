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

package tools

import (
	"reflect"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRecordCredentialsWithoutAssumeRoleReturnsStaticCredentials(t *testing.T) {
	creds, err := buildRecordCredentials(recordCredentialConfig{
		AccessKey: "source-ak",
		SecretKey: "source-sk",
	})
	require.NoError(t, err)

	value, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "source-ak", value.AccessKeyID)
	assert.Equal(t, "source-sk", value.SecretAccessKey)
	assert.Empty(t, value.SessionToken)
}

func TestBuildRecordCredentialsWithAssumeRoleWrapsSourceCredentials(t *testing.T) {
	creds, err := buildRecordCredentials(recordCredentialConfig{
		AccessKey:       "source-ak",
		SecretKey:       "source-sk",
		STSEndpoint:     "sts.example.com",
		AssumeRoleTrn:   "trn:iam::123456789012:role/target",
		RoleSessionName: "record-cli",
		DurationSeconds: 3600,
	})
	require.NoError(t, err)

	providerType := reflect.TypeOf(creds.GetProvider())
	require.NotNil(t, providerType)
	require.Equal(t, reflect.Ptr, providerType.Kind())
	assert.Equal(t, "assumeRoleProvider", providerType.Elem().Name())
	assert.Equal(t, "volcengine-provider/pkg/volcengine", providerType.Elem().PkgPath())
}

func TestRecordCredentialConfigFromViperFallsBackToLegacyRoleTrnForOIDC(t *testing.T) {
	resetRecordCredentialViper()
	defer resetRecordCredentialViper()

	viper.Set("oidc_token_file", "/tmp/token")
	viper.Set("role_trn", "trn:iam::100000000000:role/legacy-oidc")

	config := recordCredentialConfigFromViper()
	assert.Equal(t, "/tmp/token", config.OIDCTokenFile)
	assert.Equal(t, "trn:iam::100000000000:role/legacy-oidc", config.OIDCRoleTrn)
	assert.Empty(t, config.AssumeRoleTrn)
}

func TestRecordCredentialConfigFromViperSeparatesOIDCSourceAndAssumeRoleTarget(t *testing.T) {
	resetRecordCredentialViper()
	defer resetRecordCredentialViper()

	viper.Set("oidc_token_file", "/tmp/token")
	viper.Set("oidc_role_trn", "trn:iam::100000000000:role/source")
	viper.Set("role_trn", "trn:iam::200000000000:role/target")
	viper.Set("role_session_name", "record-cli")
	viper.Set("duration_seconds", int32(1800))

	config := recordCredentialConfigFromViper()
	assert.Equal(t, "/tmp/token", config.OIDCTokenFile)
	assert.Equal(t, "trn:iam::100000000000:role/source", config.OIDCRoleTrn)
	assert.Equal(t, "trn:iam::200000000000:role/target", config.AssumeRoleTrn)
	assert.Equal(t, "record-cli", config.RoleSessionName)
	assert.EqualValues(t, 1800, config.DurationSeconds)
}

func resetRecordCredentialViper() {
	for _, key := range []string{
		"access_key",
		"secret_key",
		"sts_endpoint",
		"oidc_token_file",
		"oidc_role_trn",
		"role_trn",
		"role_session_name",
	} {
		viper.Set(key, "")
	}
	viper.Set("duration_seconds", int32(0))
}
