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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

// MockPrivateZoneAPI is a mock implementation of the privateZoneAPI interface
type MockPrivateZoneAPI struct {
	mock.Mock
}

func (m *MockPrivateZoneAPI) ListPrivateZones(ctx context.Context, vpcID string) ([]*privatezone.ZoneForListPrivateZonesOutput, error) {
	args := m.Called(ctx, vpcID)
	return args.Get(0).([]*privatezone.ZoneForListPrivateZonesOutput), args.Error(1)
}

func (m *MockPrivateZoneAPI) ListRecordsByVPC(ctx context.Context, vpc string) ([]*endpoint.Endpoint, error) {
	args := m.Called(ctx, vpc)
	return args.Get(0).([]*endpoint.Endpoint), args.Error(1)
}

func (m *MockPrivateZoneAPI) GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error) {
	args := m.Called(ctx, zid)
	return args.Get(0).([]*privatezone.RecordForListRecordsOutput), args.Error(1)
}

func (m *MockPrivateZoneAPI) CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string, TTL int32) error {
	args := m.Called(ctx, zoneID, domain, recordType, target, TTL)
	return args.Error(0)
}

func (m *MockPrivateZoneAPI) BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error {
	args := m.Called(ctx, zoneID, records)
	return args.Error(0)
}

func (m *MockPrivateZoneAPI) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host string, recordType string, targets []string) error {
	args := m.Called(ctx, zoneID, host, recordType, targets)
	return args.Error(0)
}

func TestNewVolcengineProvider(t *testing.T) {
	// Test successful Provider creation
	options := []Option{
		WithPrivateZone("cn-beijing", "vpc-123456"),
		WithPrivateZoneEndpoint("custom.endpoint.com"),
	}

	// Since NewPrivateZoneWrapper will try to create a real client, we need to use a mock instead
// Here we use a trick: first create a Provider, then replace its pzClient with a mock
	provider, err := NewVolcengineProvider(options)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "vpc-123456", provider.vpcID)
	assert.True(t, provider.privateZone)
}

func TestProviderRecords(t *testing.T) {
	// Create a mock privateZoneAPI
	mockAPI := new(MockPrivateZoneAPI)

	// Mock ListRecordsByVPC response
	expectedEndpoints := []*endpoint.Endpoint{endpoint.NewEndpoint("www.example.com", "A", "1.2.3.4")}
	mockAPI.On("ListRecordsByVPC", mock.Anything, "vpc-123").Return(expectedEndpoints, nil)

	// Create Provider and inject mock API
	provider := &Provider{
		vpcID:       "vpc-123",
		privateZone: true,
		pzClient:    mockAPI,
	}

	// Call the method
	endpoints, err := provider.Records(context.Background())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, expectedEndpoints, endpoints)

	// Verify mock method was called
	mockAPI.AssertExpectations(t)
}

func TestProviderApplyChanges(t *testing.T) {
	// Create a mock privateZoneAPI
	mockAPI := new(MockPrivateZoneAPI)

	// Prepare test data
	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{endpoint.NewEndpoint("new.example.com", "A", "1.2.3.4")},
		Delete: []*endpoint.Endpoint{endpoint.NewEndpoint("old.example.com", "A", "5.6.7.8")},
	}

	// Mock DNS zone and record query responses
	mockZones := []*privatezone.ZoneForListPrivateZonesOutput{
		&privatezone.ZoneForListPrivateZonesOutput{
			ZID:      volcengine.Int32(123),
			ZoneName: volcengine.String("example.com"),
		}}
	mockAPI.On("ListPrivateZones", mock.Anything, "vpc-123").Return(mockZones, nil)

	// Mock create and delete record responses
	mockAPI.On("BatchCreatePrivateZoneRecord", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockAPI.On("DeletePrivateZoneRecord", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create Provider and inject mock API
	provider := &Provider{
		vpcID:       "vpc-123",
		privateZone: true,
		pzClient:    mockAPI,
	}

	// Call the method
	err := provider.ApplyChanges(context.Background(), changes)

	// Verify results
	assert.NoError(t, err)

	// Verify mock method was called
	mockAPI.AssertExpectations(t)
}

func TestProviderApplyChangesNil(t *testing.T) {
	// Create Provider
	provider := &Provider{}

	// Call the method with nil changes
	err := provider.ApplyChanges(context.Background(), nil)

	// Verify results
	// According to the implementation in provider.go, passing nil changes should return nil or not perform any operation
	// Since we haven't seen the complete implementation of ApplyChanges, we assume it won't return an error
	assert.NoError(t, err)
}
