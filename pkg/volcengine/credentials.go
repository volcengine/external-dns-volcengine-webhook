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
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	sdksts "github.com/volcengine/volcengine-go-sdk/service/sts"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

const (
	defaultAssumeRoleSessionName = "external-dns"
	assumeRoleRefreshWindow      = time.Minute
	assumeRolePreRefreshAhead    = 5 * time.Minute
	assumeRolePreRefreshMinDelay = 5 * time.Second
	assumeRolePreRefreshBuffer   = 30 * time.Second
	assumeRoleMinPreRefreshTTL   = 10 * time.Minute
)

type AssumeRoleOptions struct {
	Region          string
	STSEndpoint     string
	RoleTrn         string
	RoleSessionName string
	DurationSeconds int32
}

type CredentialOptions struct {
	AccessKey        string
	SecretKey        string
	STSEndpoint      string
	OIDCTokenFile    string
	OIDCRoleTrn      string
	AssumeRoleConfig *AssumeRoleOptions
}

type assumeRoleAPI interface {
	AssumeRole(input *sdksts.AssumeRoleInput) (*sdksts.AssumeRoleOutput, error)
}

type assumeRoleClientFactory func(source credentials.Value, region, endpoint string) assumeRoleAPI

type refreshTimer interface {
	Stop() bool
}

type assumeRoleProvider struct {
	sourceCredentials *credentials.Credentials
	options           AssumeRoleOptions
	clientFactory     assumeRoleClientFactory
	now               func() time.Time
	afterFunc         func(time.Duration, func()) refreshTimer
	logger            logrus.FieldLogger

	mu             sync.Mutex
	cond           *sync.Cond
	current        credentials.Value
	actualExpiry   time.Time
	lazyExpiry     time.Time
	publishedNonce uint64
	servedNonce    uint64
	refreshing     bool
	timer          refreshTimer
}

func NewStaticCredentials(accessKey, secretKey string) *credentials.Credentials {
	return credentials.NewStaticCredentials(accessKey, secretKey, "")
}

func NewOIDCCredentials(stsEndpoint, oidcRoleTrn, oidcTokenFilePath string) *credentials.Credentials {
	if stsEndpoint == "" {
		stsEndpoint = defaultStsEndpoint
	}

	p := credentials.NewOIDCCredentialsProviderFromEnv()
	p.OIDCTokenFilePath = oidcTokenFilePath
	p.RoleTrn = oidcRoleTrn
	p.Endpoint = stsEndpoint
	p.RoleSessionName = defaultAssumeRoleSessionName

	return credentials.NewCredentials(p)
}

func NewCredentials(options CredentialOptions) (*credentials.Credentials, error) {
	var sourceCredentials *credentials.Credentials

	switch {
	case options.AccessKey != "" && options.SecretKey != "":
		sourceCredentials = NewStaticCredentials(options.AccessKey, options.SecretKey)
	case options.OIDCTokenFile != "" && options.OIDCRoleTrn != "":
		sourceCredentials = NewOIDCCredentials(options.STSEndpoint, options.OIDCRoleTrn, options.OIDCTokenFile)
	default:
		return nil, fmt.Errorf("access_key/secret_key or oidc_token_file/oidc_role_trn is required")
	}

	return NewAssumeRoleCredentials(sourceCredentials, options.AssumeRoleConfig)
}

func NewAssumeRoleCredentials(sourceCredentials *credentials.Credentials, options *AssumeRoleOptions) (*credentials.Credentials, error) {
	if options == nil || options.RoleTrn == "" {
		return sourceCredentials, nil
	}
	if sourceCredentials == nil {
		return nil, fmt.Errorf("source credentials are required when assume role is enabled")
	}

	normalized := *options
	if normalized.STSEndpoint == "" {
		normalized.STSEndpoint = defaultStsEndpoint
	}
	if normalized.RoleSessionName == "" {
		normalized.RoleSessionName = defaultAssumeRoleSessionName
	}

	provider := &assumeRoleProvider{
		sourceCredentials: sourceCredentials,
		options:           normalized,
		clientFactory:     newAssumeRoleClient,
		now:               time.Now,
		afterFunc: func(d time.Duration, f func()) refreshTimer {
			return time.AfterFunc(d, f)
		},
		logger: logrus.WithFields(logrus.Fields{
			"component":         "assume-role-credentials",
			"role_trn":          normalized.RoleTrn,
			"role_session_name": normalized.RoleSessionName,
		}),
	}
	provider.cond = sync.NewCond(&provider.mu)

	return credentials.NewExpireAbleCredentials(provider), nil
}

func (p *assumeRoleProvider) Retrieve() (credentials.Value, error) {
	for {
		p.mu.Lock()
		now := p.now()
		if p.hasPublishedRefreshLocked(now) {
			p.servedNonce = p.publishedNonce
			value := p.current
			p.mu.Unlock()
			return value, nil
		}
		if p.refreshing {
			p.cond.Wait()
			p.mu.Unlock()
			continue
		}
		p.refreshing = true
		p.mu.Unlock()

		value, actualExpiry, err := p.fetchCredentials("sync")

		p.mu.Lock()
		p.refreshing = false
		if err != nil {
			p.cond.Broadcast()
			p.mu.Unlock()
			p.logger.WithError(err).Error("failed to refresh assume role credentials on demand")
			return credentials.Value{}, err
		}

		delay := p.updateCacheLocked(value, actualExpiry, true)
		p.cond.Broadcast()
		p.mu.Unlock()

		p.logger.WithFields(logrus.Fields{
			"expires_at":      actualExpiry.UTC().Format(time.RFC3339),
			"lazy_expires_at": actualExpiry.Add(-assumeRoleRefreshWindow).UTC().Format(time.RFC3339),
			"pre_refresh_in":  delay.String(),
		}).Info("refreshed assume role credentials on demand")

		return value, nil
	}
}

func (p *assumeRoleProvider) IsExpired() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.hasCachedValueLocked() {
		return true
	}
	if p.publishedNonce != p.servedNonce {
		return true
	}
	return p.lazyExpiry.Before(p.now())
}

func (p *assumeRoleProvider) ExpiresAt() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.lazyExpiry
}

func (p *assumeRoleProvider) hasCachedValueLocked() bool {
	return p.current.ProviderName != ""
}

func (p *assumeRoleProvider) hasPublishedRefreshLocked(now time.Time) bool {
	return p.hasCachedValueLocked() && p.publishedNonce != p.servedNonce && !p.lazyExpiry.Before(now)
}

func (p *assumeRoleProvider) updateCacheLocked(value credentials.Value, actualExpiry time.Time, markServed bool) time.Duration {
	p.current = value
	p.actualExpiry = actualExpiry
	p.lazyExpiry = actualExpiry.Add(-assumeRoleRefreshWindow)
	p.publishedNonce++
	if markServed {
		p.servedNonce = p.publishedNonce
	}
	return p.schedulePreRefreshLocked(actualExpiry)
}

func (p *assumeRoleProvider) schedulePreRefreshLocked(actualExpiry time.Time) time.Duration {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}

	delay := computeAssumeRolePreRefreshDelay(p.now(), actualExpiry)
	if delay <= 0 {
		return 0
	}

	p.timer = p.afterFunc(delay, p.refreshInBackground)
	return delay
}

func (p *assumeRoleProvider) refreshInBackground() {
	p.mu.Lock()
	p.timer = nil
	now := p.now()
	if !p.hasCachedValueLocked() || !now.Before(p.actualExpiry) {
		p.mu.Unlock()
		return
	}
	if p.refreshing {
		p.mu.Unlock()
		return
	}
	p.refreshing = true
	p.mu.Unlock()

	value, actualExpiry, err := p.fetchCredentials("async")

	p.mu.Lock()
	p.refreshing = false
	if err != nil {
		retryDelay := p.scheduleRetryLocked()
		remainingTTL := time.Duration(0)
		if p.hasCachedValueLocked() && now.Before(p.actualExpiry) {
			remainingTTL = p.actualExpiry.Sub(now)
		}
		p.cond.Broadcast()
		p.mu.Unlock()

		fields := logrus.Fields{
			"remaining_ttl": remainingTTL.String(),
		}
		if retryDelay > 0 {
			fields["next_retry_in"] = retryDelay.String()
		}
		p.logger.WithFields(fields).WithError(err).Warn("failed to pre-refresh assume role credentials")
		return
	}

	delay := p.updateCacheLocked(value, actualExpiry, false)
	p.cond.Broadcast()
	p.mu.Unlock()

	p.logger.WithFields(logrus.Fields{
		"expires_at":      actualExpiry.UTC().Format(time.RFC3339),
		"lazy_expires_at": actualExpiry.Add(-assumeRoleRefreshWindow).UTC().Format(time.RFC3339),
		"pre_refresh_in":  delay.String(),
	}).Info("pre-refreshed assume role credentials")
}

func (p *assumeRoleProvider) scheduleRetryLocked() time.Duration {
	if !p.hasCachedValueLocked() {
		return 0
	}

	now := p.now()
	if !now.Before(p.actualExpiry) {
		return 0
	}

	remainingTTL := p.actualExpiry.Sub(now)
	retryDelay := remainingTTL / 2
	if retryDelay > time.Minute {
		retryDelay = time.Minute
	}
	if retryDelay < assumeRolePreRefreshMinDelay {
		retryDelay = assumeRolePreRefreshMinDelay
	}
	if retryDelay >= remainingTTL {
		retryDelay = remainingTTL - time.Second
	}
	if retryDelay <= 0 {
		return 0
	}

	if p.timer != nil {
		p.timer.Stop()
	}
	p.timer = p.afterFunc(retryDelay, p.refreshInBackground)
	return retryDelay
}

func (p *assumeRoleProvider) fetchCredentials(mode string) (credentials.Value, time.Time, error) {
	start := p.now()

	sourceValue, err := p.sourceCredentials.Get()
	if err != nil {
		return credentials.Value{}, time.Time{}, fmt.Errorf("retrieve source credentials: %w", err)
	}

	client := p.clientFactory(sourceValue, p.options.Region, p.options.STSEndpoint)
	req := &sdksts.AssumeRoleInput{}
	req.SetRoleTrn(p.options.RoleTrn)
	req.SetRoleSessionName(p.options.RoleSessionName)
	if p.options.DurationSeconds > 0 {
		req.SetDurationSeconds(p.options.DurationSeconds)
	}

	resp, err := client.AssumeRole(req)
	if err != nil {
		return credentials.Value{}, time.Time{}, fmt.Errorf("assume role failed: %w", err)
	}
	if resp == nil || resp.Metadata == nil || resp.Metadata.Error != nil || resp.Credentials == nil {
		return credentials.Value{}, time.Time{}, fmt.Errorf("assume role returned invalid response: %+v", resp)
	}

	expiration, err := parseAssumeRoleExpiration(volcengine.StringValue(resp.Credentials.ExpiredTime), p.now(), p.options.DurationSeconds)
	if err != nil {
		return credentials.Value{}, time.Time{}, err
	}

	p.logger.WithFields(logrus.Fields{
		"mode":        mode,
		"latency":     p.now().Sub(start).String(),
		"expires_at":  expiration.UTC().Format(time.RFC3339),
		"session_ttl": expiration.Sub(p.now()).String(),
	}).Debug("assume role call succeeded")

	return credentials.Value{
		AccessKeyID:     volcengine.StringValue(resp.Credentials.AccessKeyId),
		SecretAccessKey: volcengine.StringValue(resp.Credentials.SecretAccessKey),
		SessionToken:    volcengine.StringValue(resp.Credentials.SessionToken),
		ProviderName:    "AssumeRoleProvider",
	}, expiration, nil
}

func newAssumeRoleClient(source credentials.Value, region, endpoint string) assumeRoleAPI {
	cfg := volcengine.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(source.AccessKeyID, source.SecretAccessKey, source.SessionToken)).
		WithRegion(region)
	if endpoint != "" {
		cfg = cfg.WithEndpoint(endpoint)
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return &failedAssumeRoleClient{err: fmt.Errorf("create sts session: %w", err)}
	}
	return sdksts.New(sess)
}

type failedAssumeRoleClient struct {
	err error
}

func (c *failedAssumeRoleClient) AssumeRole(input *sdksts.AssumeRoleInput) (*sdksts.AssumeRoleOutput, error) {
	return nil, c.err
}

func parseAssumeRoleExpiration(expiration string, now time.Time, durationSeconds int32) (time.Time, error) {
	if expiration != "" {
		expiredAt, err := time.Parse(time.RFC3339, expiration)
		if err == nil {
			return expiredAt, nil
		}
	}
	if durationSeconds > 0 {
		return now.Add(time.Duration(durationSeconds) * time.Second), nil
	}
	return time.Time{}, fmt.Errorf("failed to parse assume role expiration %q", expiration)
}

func computeAssumeRolePreRefreshDelay(now, expiration time.Time) time.Duration {
	ttl := expiration.Sub(now)
	if ttl <= 0 || ttl <= assumeRoleMinPreRefreshTTL {
		return 0
	}

	lead := assumeRolePreRefreshAhead
	maxLead := ttl - assumeRolePreRefreshMinDelay
	if maxLead <= 0 {
		return 0
	}
	if lead > maxLead {
		lead = maxLead
	}

	minLead := assumeRoleRefreshWindow + assumeRolePreRefreshBuffer
	if ttl > minLead+assumeRolePreRefreshMinDelay && lead < minLead {
		lead = minLead
	}

	delay := ttl - lead
	if delay < assumeRolePreRefreshMinDelay && ttl > assumeRolePreRefreshMinDelay {
		delay = assumeRolePreRefreshMinDelay
	}
	if delay < 0 {
		return 0
	}
	return delay
}
