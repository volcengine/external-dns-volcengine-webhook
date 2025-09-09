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

package volcengine

import (
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
)

func WithPrivateZone(region, vpcId string) Option {
	return func(c *Config) {
		c.RegionID = region
		c.VpcId = vpcId
		c.PrivateZone = true
	}
}

func WithPrivateZoneEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.PrivateZoneEndpoint = endpoint
	}
}

func WithStaticCredentials(accessKey, secretKey string) Option {
	return func(c *Config) {
		c.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}
}

func WithOIDCCredentials(stsEndpoint, oidcRoleTrn, oidcTokenFilePath string) Option {
	if stsEndpoint == "" {
		stsEndpoint = defaultStsEndpoint
	}
	return func(c *Config) {
		p := credentials.NewOIDCCredentialsProviderFromEnv()
		p.OIDCTokenFilePath = oidcTokenFilePath
		p.RoleTrn = oidcRoleTrn
		p.Endpoint = stsEndpoint
		p.RoleSessionName = "external-dns"

		c.Credentials = credentials.NewCredentials(p)
	}
}
