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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdksts "github.com/volcengine/volcengine-go-sdk/service/sts"
	sdkcredentials "github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/response"
)

type stubSourceProvider struct {
	value sdkcredentials.Value
}

func stringPtr(v string) *string {
	return &v
}

func (p *stubSourceProvider) Retrieve() (sdkcredentials.Value, error) {
	return p.value, nil
}

func (p *stubSourceProvider) IsExpired() bool {
	return false
}

type fakeAssumeRoleAPI struct {
	t                *testing.T
	expectedRoleTrn  string
	expectedSession  string
	expectedDuration int32
	response         *sdksts.AssumeRoleOutput
	err              error
	beforeReturn     func()
}

func (f *fakeAssumeRoleAPI) AssumeRole(req *sdksts.AssumeRoleInput) (*sdksts.AssumeRoleOutput, error) {
	if f.expectedRoleTrn == "" {
		assert.Nil(f.t, req.RoleTrn)
	} else {
		assert.Equal(f.t, f.expectedRoleTrn, *req.RoleTrn)
	}
	if f.expectedSession == "" {
		assert.Nil(f.t, req.RoleSessionName)
	} else {
		assert.Equal(f.t, f.expectedSession, *req.RoleSessionName)
	}
	if f.expectedDuration == 0 {
		assert.Nil(f.t, req.DurationSeconds)
	} else {
		assert.EqualValues(f.t, f.expectedDuration, *req.DurationSeconds)
	}
	if f.beforeReturn != nil {
		f.beforeReturn()
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

type fakeScheduledTimer struct {
	stopped bool
}

func (t *fakeScheduledTimer) Stop() bool {
	wasStopped := t.stopped
	t.stopped = true
	return !wasStopped
}

type fakeTimerRegistration struct {
	delay time.Duration
	fn    func()
	timer *fakeScheduledTimer
}

func TestNewAssumeRoleCredentialsWithoutRoleTrnReturnsSourceCredentials(t *testing.T) {
	source := NewStaticCredentials("ak", "sk")

	creds, err := NewAssumeRoleCredentials(source, nil)
	require.NoError(t, err)
	assert.Same(t, source, creds)

	creds, err = NewAssumeRoleCredentials(source, &AssumeRoleOptions{})
	require.NoError(t, err)
	assert.Same(t, source, creds)
}

func TestNewAssumeRoleCredentialsWithStaticSource(t *testing.T) {
	source := NewStaticCredentials("source-ak", "source-sk")
	now := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		STSEndpoint:     "sts.example.com",
		RoleTrn:         "trn:iam::123456789012:role/target",
		RoleSessionName: "custom-session",
		DurationSeconds: 3600,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return now }
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		assert.Equal(t, "source-ak", sourceValue.AccessKeyID)
		assert.Equal(t, "source-sk", sourceValue.SecretAccessKey)
		assert.Empty(t, sourceValue.SessionToken)
		assert.Equal(t, "cn-beijing", region)
		assert.Equal(t, "sts.example.com", endpoint)
		return &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::123456789012:role/target",
			expectedSession:  "custom-session",
			expectedDuration: 3600,
			response: &sdksts.AssumeRoleOutput{
				Metadata: &response.ResponseMetadata{},
				Credentials: &sdksts.CredentialsForAssumeRoleOutput{
					AccessKeyId:     stringPtr("target-ak"),
					SecretAccessKey: stringPtr("target-sk"),
					SessionToken:    stringPtr("target-token"),
					ExpiredTime:     stringPtr(now.Add(time.Hour).Format(time.RFC3339)),
				},
			},
		}
	}

	value, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "target-ak", value.AccessKeyID)
	assert.Equal(t, "target-sk", value.SecretAccessKey)
	assert.Equal(t, "target-token", value.SessionToken)

	expiresAt, err := creds.ExpiresAt()
	require.NoError(t, err)
	assert.Equal(t, now.Add(59*time.Minute), expiresAt)
}

func TestNewAssumeRoleCredentialsWithOIDCSource(t *testing.T) {
	now := time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC)
	source := sdkcredentials.NewCredentials(&stubSourceProvider{
		value: sdkcredentials.Value{
			AccessKeyID:     "oidc-ak",
			SecretAccessKey: "oidc-sk",
			SessionToken:    "oidc-token",
			ProviderName:    "OIDC",
		},
	})

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		RoleTrn:         "trn:iam::210987654321:role/oidc-target",
		DurationSeconds: 1800,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return now }
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		assert.Equal(t, "oidc-ak", sourceValue.AccessKeyID)
		assert.Equal(t, "oidc-sk", sourceValue.SecretAccessKey)
		assert.Equal(t, "oidc-token", sourceValue.SessionToken)
		assert.Equal(t, "cn-beijing", region)
		assert.Equal(t, defaultStsEndpoint, endpoint)
		return &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::210987654321:role/oidc-target",
			expectedSession:  defaultAssumeRoleSessionName,
			expectedDuration: 1800,
			response: &sdksts.AssumeRoleOutput{
				Metadata: &response.ResponseMetadata{},
				Credentials: &sdksts.CredentialsForAssumeRoleOutput{
					AccessKeyId:     stringPtr("oidc-target-ak"),
					SecretAccessKey: stringPtr("oidc-target-sk"),
					SessionToken:    stringPtr("oidc-target-token"),
				},
			},
		}
	}

	value, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "oidc-target-ak", value.AccessKeyID)
	assert.Equal(t, "oidc-target-sk", value.SecretAccessKey)
	assert.Equal(t, "oidc-target-token", value.SessionToken)

	expiresAt, err := creds.ExpiresAt()
	require.NoError(t, err)
	assert.Equal(t, now.Add(29*time.Minute), expiresAt)
}

func TestAssumeRoleCredentialsExpireForcesRefresh(t *testing.T) {
	source := NewStaticCredentials("source-ak", "source-sk")
	now := time.Date(2026, 5, 20, 11, 30, 0, 0, time.UTC)

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		RoleTrn:         "trn:iam::123456789012:role/target",
		RoleSessionName: "expire-session",
		DurationSeconds: 3600,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return now }

	callCount := 0
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		callCount++
		return &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::123456789012:role/target",
			expectedSession:  "expire-session",
			expectedDuration: 3600,
			response: &sdksts.AssumeRoleOutput{
				Metadata: &response.ResponseMetadata{},
				Credentials: &sdksts.CredentialsForAssumeRoleOutput{
					AccessKeyId:     stringPtr("target-ak"),
					SecretAccessKey: stringPtr("target-sk"),
					SessionToken:    stringPtr(string(rune('0' + callCount))),
					ExpiredTime:     stringPtr(now.Add(time.Duration(callCount) * time.Hour).Format(time.RFC3339)),
				},
			},
		}
	}

	firstValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "1", firstValue.SessionToken)
	assert.Equal(t, 1, callCount)

	creds.Expire()

	secondValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "2", secondValue.SessionToken)
	assert.Equal(t, 2, callCount)
}

func TestAssumeRoleCredentialsPreRefreshPublishesNewCredentials(t *testing.T) {
	source := NewStaticCredentials("source-ak", "source-sk")
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	currentTime := now

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		RoleTrn:         "trn:iam::123456789012:role/target",
		RoleSessionName: "prefetch-session",
		DurationSeconds: 3600,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return currentTime }

	var timers []fakeTimerRegistration
	provider.afterFunc = func(d time.Duration, f func()) refreshTimer {
		timer := &fakeScheduledTimer{}
		timers = append(timers, fakeTimerRegistration{
			delay: d,
			fn:    f,
			timer: timer,
		})
		return timer
	}

	callCount := 0
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		callCount++
		expiredAt := now.Add(time.Duration(callCount) * time.Hour)
		return &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::123456789012:role/target",
			expectedSession:  "prefetch-session",
			expectedDuration: 3600,
			response: &sdksts.AssumeRoleOutput{
				Metadata: &response.ResponseMetadata{},
				Credentials: &sdksts.CredentialsForAssumeRoleOutput{
					AccessKeyId:     stringPtr("target-ak"),
					SecretAccessKey: stringPtr("target-sk"),
					SessionToken:    stringPtr(string(rune('0' + callCount))),
					ExpiredTime:     stringPtr(expiredAt.Format(time.RFC3339)),
				},
			},
		}
	}

	firstValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "1", firstValue.SessionToken)
	require.Len(t, timers, 1)
	assert.Equal(t, 55*time.Minute, timers[0].delay)

	currentTime = now.Add(timers[0].delay)
	timers[0].fn()

	require.Len(t, timers, 2)
	assert.Equal(t, time.Hour, timers[1].delay)
	assert.Equal(t, 2, callCount)

	secondValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "2", secondValue.SessionToken)
	assert.Equal(t, 2, callCount)

	expiresAt, err := creds.ExpiresAt()
	require.NoError(t, err)
	assert.Equal(t, now.Add(119*time.Minute), expiresAt)
}

func TestAssumeRoleCredentialsPreRefreshFailureKeepsServingCachedCredentials(t *testing.T) {
	source := NewStaticCredentials("source-ak", "source-sk")
	now := time.Date(2026, 5, 20, 13, 0, 0, 0, time.UTC)
	currentTime := now

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		RoleTrn:         "trn:iam::123456789012:role/target",
		RoleSessionName: "prefetch-session",
		DurationSeconds: 3600,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return currentTime }

	var timers []fakeTimerRegistration
	provider.afterFunc = func(d time.Duration, f func()) refreshTimer {
		timer := &fakeScheduledTimer{}
		timers = append(timers, fakeTimerRegistration{
			delay: d,
			fn:    f,
			timer: timer,
		})
		return timer
	}

	callCount := 0
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		callCount++
		if callCount == 1 {
			return &fakeAssumeRoleAPI{
				t:                t,
				expectedRoleTrn:  "trn:iam::123456789012:role/target",
				expectedSession:  "prefetch-session",
				expectedDuration: 3600,
				response: &sdksts.AssumeRoleOutput{
					Metadata: &response.ResponseMetadata{},
					Credentials: &sdksts.CredentialsForAssumeRoleOutput{
						AccessKeyId:     stringPtr("target-ak"),
						SecretAccessKey: stringPtr("target-sk"),
						SessionToken:    stringPtr("cached-token"),
						ExpiredTime:     stringPtr(now.Add(time.Hour).Format(time.RFC3339)),
					},
				},
			}
		}
		return &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::123456789012:role/target",
			expectedSession:  "prefetch-session",
			expectedDuration: 3600,
			err:              errors.New("sts unavailable"),
		}
	}

	initialValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "cached-token", initialValue.SessionToken)
	require.Len(t, timers, 1)

	currentTime = now.Add(55 * time.Minute)
	timers[0].fn()

	assert.Equal(t, 2, callCount)
	require.Len(t, timers, 2)
	assert.Equal(t, time.Minute, timers[1].delay)

	valueAfterFailure, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "cached-token", valueAfterFailure.SessionToken)
	assert.Equal(t, 2, callCount)
}

func TestAssumeRoleCredentialsShortTTLFallsBackToLazyRefresh(t *testing.T) {
	source := NewStaticCredentials("source-ak", "source-sk")
	now := time.Date(2026, 5, 20, 14, 0, 0, 0, time.UTC)
	currentTime := now

	creds, err := NewAssumeRoleCredentials(source, &AssumeRoleOptions{
		Region:          "cn-beijing",
		RoleTrn:         "trn:iam::123456789012:role/target",
		RoleSessionName: "prefetch-session",
		DurationSeconds: 120,
	})
	require.NoError(t, err)

	provider, ok := creds.GetProvider().(*assumeRoleProvider)
	require.True(t, ok)
	provider.now = func() time.Time { return currentTime }

	var timers []fakeTimerRegistration
	provider.afterFunc = func(d time.Duration, f func()) refreshTimer {
		timer := &fakeScheduledTimer{}
		timers = append(timers, fakeTimerRegistration{
			delay: d,
			fn:    f,
			timer: timer,
		})
		return timer
	}

	blockRefresh := make(chan struct{})
	var mu sync.Mutex
	callCount := 0
	provider.clientFactory = func(sourceValue sdkcredentials.Value, region, endpoint string) assumeRoleAPI {
		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		expiredAt := now.Add(time.Duration(currentCall) * 2 * time.Minute)
		api := &fakeAssumeRoleAPI{
			t:                t,
			expectedRoleTrn:  "trn:iam::123456789012:role/target",
			expectedSession:  "prefetch-session",
			expectedDuration: 120,
			response: &sdksts.AssumeRoleOutput{
				Metadata: &response.ResponseMetadata{},
				Credentials: &sdksts.CredentialsForAssumeRoleOutput{
					AccessKeyId:     stringPtr("target-ak"),
					SecretAccessKey: stringPtr("target-sk"),
					SessionToken:    stringPtr(string(rune('0' + currentCall))),
					ExpiredTime:     stringPtr(expiredAt.Format(time.RFC3339)),
				},
			},
		}
		if currentCall == 2 {
			api.beforeReturn = func() {
				<-blockRefresh
			}
		}
		return api
	}

	firstValue, err := creds.Get()
	require.NoError(t, err)
	assert.Equal(t, "1", firstValue.SessionToken)
	assert.Empty(t, timers)

	currentTime = now.Add(61 * time.Second)
	done := make(chan sdkcredentials.Value, 1)
	errCh := make(chan error, 1)
	go func() {
		value, getErr := creds.Get()
		if getErr != nil {
			errCh <- getErr
			return
		}
		done <- value
	}()

	time.Sleep(20 * time.Millisecond)
	close(blockRefresh)

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case value := <-done:
		assert.Equal(t, "2", value.SessionToken)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for lazy credential refresh")
	}

	mu.Lock()
	assert.Equal(t, 2, callCount)
	mu.Unlock()
}

func TestComputeAssumeRolePreRefreshDelayDisablesShortTTL(t *testing.T) {
	now := time.Date(2026, 5, 20, 15, 0, 0, 0, time.UTC)

	assert.Zero(t, computeAssumeRolePreRefreshDelay(now, now.Add(2*time.Minute)))
	assert.Zero(t, computeAssumeRolePreRefreshDelay(now, now.Add(10*time.Minute)))
	assert.Equal(t, 6*time.Minute, computeAssumeRolePreRefreshDelay(now, now.Add(11*time.Minute)))
}
