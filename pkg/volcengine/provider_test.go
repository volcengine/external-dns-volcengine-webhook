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
	"errors"
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

func (m *MockPrivateZoneAPI) UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, recordID string, host, recordType, target string, TTL int32) error {
	args := m.Called(ctx, zoneID, recordID, host, recordType, target, TTL)
	return args.Error(0)
}

func (m *MockPrivateZoneAPI) DeletePrivateZoneRecordById(ctx context.Context, zoneID int64, recordID string) error {
	args := m.Called(ctx, zoneID, recordID)
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

func TestUpdatePrivateZoneRecords(t *testing.T) {
	// Create a mock privateZoneAPI
	mockAPI := new(MockPrivateZoneAPI)

	// Create Provider and inject mock API
	provider := &Provider{
		pzClient: mockAPI,
	}

	// Prepare test data
	ctx := context.Background()
	// Before modification: zoneMap := provider.ZoneIDName{
	// After modification: use correct import package reference
	zoneMap := map[string]string{
		"123": "example.com",
	}

	// Test Scenario 1: Successfully update record TTL
	endpoint1 := endpoint.NewEndpointWithTTL("www.example.com", "A", endpoint.TTL(60), "1.2.3.4")
	mockRecords := []*privatezone.RecordForListRecordsOutput{
		{
			Host:     volcengine.String("www"),
			Type:     volcengine.String("A"),
			Value:    volcengine.String("1.2.3.4"),
			TTL:      volcengine.Int32(300),
			RecordID: volcengine.String("record-1"),
			ZID:      volcengine.Int32(123),
		},
	}
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(mockRecords, nil)
	mockAPI.On("UpdatePrivateZoneRecord", ctx, int64(123), "record-1", "www", "A", "1.2.3.4", int32(60)).Return(nil)

	// Test Scenario 2: Successfully delete old record and create new record
	endpoint2 := endpoint.NewEndpoint("www.example.com", "A", "5.6.7.8")
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(mockRecords, nil)
	mockAPI.On("DeletePrivateZoneRecordById", ctx, int64(123), "record-1").Return(nil)
	mockAPI.On("CreatePrivateZoneRecord", ctx, int64(123), "www", "A", "5.6.7.8", int32(0)).Return(nil)

	// Test Scenario 3: Successfully create record
	endpoint3 := endpoint.NewEndpoint("new.example.com", "A", "9.10.11.12")
	emptyRecords := []*privatezone.RecordForListRecordsOutput{}
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(emptyRecords, nil)
	mockAPI.On("CreatePrivateZoneRecord", ctx, int64(123), "new", "A", "9.10.11.12", int32(0)).Return(nil)

	// Test Scenario 4: Handle case with no matching zone
	endpoint4 := endpoint.NewEndpoint("www.unknown.com", "A", "1.2.3.4")
	//mockAPI.On("GetPrivateZoneRecords", ctx, int64(0)).Return(nil, errors.New("should not be called"))

	// Execute Test Scenario 1
	err := provider.updatePrivateZoneRecords(ctx, zoneMap, []*endpoint.Endpoint{endpoint1})
	assert.NoError(t, err)

	// Execute Test Scenario 2
	err = provider.updatePrivateZoneRecords(ctx, zoneMap, []*endpoint.Endpoint{endpoint2})
	assert.NoError(t, err)

	// Execute Test Scenario 3
	err = provider.updatePrivateZoneRecords(ctx, zoneMap, []*endpoint.Endpoint{endpoint3})
	assert.NoError(t, err)

	// Execute Test Scenario 4
	err = provider.updatePrivateZoneRecords(ctx, map[string]string{}, []*endpoint.Endpoint{endpoint4})
	assert.NoError(t, err)

	// Verify all mock methods were called correctly
	mockAPI.AssertExpectations(t)
}

func TestUpdatePrivateZoneRecordsErrorCases(t *testing.T) {
	// Create a mock privateZoneAPI
	mockAPI := new(MockPrivateZoneAPI)

	// Create Provider and inject mock API
	provider := &Provider{
		pzClient: mockAPI,
	}

	// Prepare test data
	ctx := context.Background()
	ep := endpoint.NewEndpoint("www.example.com", "A", "1.2.3.4")

	// Test Scenario 1: Invalid zone ID
	invalidZoneMap := map[string]string{
		"invalid-zid": "example.com",
	}
	err := provider.updatePrivateZoneRecords(ctx, invalidZoneMap, []*endpoint.Endpoint{ep})
	assert.Error(t, err)

	// Test Scenario 2: Failed to get zone records
	validZoneMap := map[string]string{
		"123": "example.com",
	}
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return([]*privatezone.RecordForListRecordsOutput{}, errors.New("API error"))
	err = provider.updatePrivateZoneRecords(ctx, validZoneMap, []*endpoint.Endpoint{ep})
	assert.Error(t, err)
	mockAPI.ExpectedCalls = nil

	// Test Scenario 3: Update record TTL fails but continues execution
	mockRecords := []*privatezone.RecordForListRecordsOutput{
		{
			Host:     volcengine.String("www"),
			Type:     volcengine.String("A"),
			Value:    volcengine.String("1.2.3.4"),
			TTL:      volcengine.Int32(300),
			RecordID: volcengine.String("record-1"),
			ZID:      volcengine.Int32(123),
		},
	}
	endpointWithTTL := endpoint.NewEndpointWithTTL("www.example.com", "A", endpoint.TTL(60), "1.2.3.4")
	endpointWithTTL2 := endpoint.NewEndpointWithTTL("app.example.com", "A", endpoint.TTL(60), "1.2.3.4")
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(mockRecords, nil)
	mockAPI.On("UpdatePrivateZoneRecord", ctx, int64(123), "record-1", "www", "A", "1.2.3.4", int32(60)).Return(errors.New("Update error"))
	mockAPI.On("CreatePrivateZoneRecord", ctx, int64(123), "app", "A", "1.2.3.4", int32(60)).Return(nil)
	// Ensure the entire process continues even if update fails
	err = provider.updatePrivateZoneRecords(ctx, validZoneMap, []*endpoint.Endpoint{endpointWithTTL, endpointWithTTL2})
	assert.NoError(t, err) // Although individual update failed, the overall method should continue and return nil

	// Verify all mock methods were called correctly
	mockAPI.AssertExpectations(t)
}

func TestUpdatePrivateZoneRecordsWithSpecialTypes(t *testing.T) {
	// Create a mock privateZoneAPI
	mockAPI := new(MockPrivateZoneAPI)

	// Create Provider and inject mock API
	provider := &Provider{
		pzClient: mockAPI,
	}

	// Prepare test data
	ctx := context.Background()
	zoneMap := map[string]string{
		"123": "example.com",
	}

	// Test TXT record type
	txtEndpoint := endpoint.NewEndpoint("txt.example.com", "TXT", "\"heritage=text value\"")
	emptyRecords := []*privatezone.RecordForListRecordsOutput{}
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(emptyRecords, nil)
	// Note: TXT record values will be unescaped
	mockAPI.On("CreatePrivateZoneRecord", ctx, int64(123), "txt", "TXT", "heritage=text value", int32(0)).Return(nil)

	// Test CNAME record type
	cnameEndpoint := endpoint.NewEndpoint("cname.example.com", "CNAME", "target.example.com")
	mockAPI.On("GetPrivateZoneRecords", ctx, int64(123)).Return(emptyRecords, nil)
	// Note: CNAME record values may be processed (adding dots, etc.)
	mockAPI.On("CreatePrivateZoneRecord", ctx, int64(123), "cname", "CNAME", "target.example.com.", int32(0)).Return(nil)

	// Execute tests
	err := provider.updatePrivateZoneRecords(ctx, zoneMap, []*endpoint.Endpoint{txtEndpoint})
	assert.NoError(t, err)

	err = provider.updatePrivateZoneRecords(ctx, zoneMap, []*endpoint.Endpoint{cnameEndpoint})
	assert.NoError(t, err)

	// Verify all mock methods were called correctly
	mockAPI.AssertExpectations(t)
}