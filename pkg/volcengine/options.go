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

func WithOIDCCredentials(stsEndpoint, roleTrn, sessionTokenFile string) Option {
	if stsEndpoint == "" {
		stsEndpoint = defaultEndpoint
	}
	return func(c *Config) {
		p := credentials.NewOIDCCredentialsProviderFromEnv()
		p.OIDCTokenFilePath = sessionTokenFile
		p.RoleTrn = roleTrn
		p.Endpoint = stsEndpoint
		p.RoleSessionName = "external-dns"
		
		c.Credentials = credentials.NewCredentials(p)
	}
}
