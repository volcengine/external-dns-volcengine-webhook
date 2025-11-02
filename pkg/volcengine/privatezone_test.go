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
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/response"
)

// MockClient is a simple mock client that only implements the methods we need in our tests
type MockClient struct {
	ListPrivateZonesFunc  func(ctx context.Context, input *privatezone.ListPrivateZonesInput) (*privatezone.ListPrivateZonesOutput, error)
	ListRecordsFunc       func(ctx context.Context, input *privatezone.ListRecordsInput) (*privatezone.ListRecordsOutput, error)
	CreateRecordFunc      func(ctx context.Context, input *privatezone.CreateRecordInput) (*privatezone.CreateRecordOutput, error)
	BatchCreateRecordFunc func(ctx context.Context, input *privatezone.BatchCreateRecordInput) (*privatezone.BatchCreateRecordOutput, error)
	BatchDeleteRecordFunc func(ctx context.Context, input *privatezone.BatchDeleteRecordInput) (*privatezone.BatchDeleteRecordOutput, error)
	UpdateRecordFunc      func(ctx context.Context, input *privatezone.UpdateRecordInput) (*privatezone.UpdateRecordOutput, error)
	DeleteRecordFunc      func(ctx context.Context, input *privatezone.DeleteRecordInput) (*privatezone.DeleteRecordOutput, error)
}

// Implement necessary methods to match the privateZoneClient interface
func (m *MockClient) ListPrivateZonesWithContext(ctx context.Context, input *privatezone.ListPrivateZonesInput, options ...request.Option) (*privatezone.ListPrivateZonesOutput, error) {
	if m.ListPrivateZonesFunc != nil {
		return m.ListPrivateZonesFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) ListRecordsWithContext(ctx context.Context, input *privatezone.ListRecordsInput, options ...request.Option) (*privatezone.ListRecordsOutput, error) {
	if m.ListRecordsFunc != nil {
		return m.ListRecordsFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) CreateRecordWithContext(ctx context.Context, input *privatezone.CreateRecordInput, options ...request.Option) (*privatezone.CreateRecordOutput, error) {
	if m.CreateRecordFunc != nil {
		return m.CreateRecordFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) BatchCreateRecordWithContext(ctx context.Context, input *privatezone.BatchCreateRecordInput, options ...request.Option) (*privatezone.BatchCreateRecordOutput, error) {
	if m.BatchCreateRecordFunc != nil {
		return m.BatchCreateRecordFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) BatchDeleteRecordWithContext(ctx context.Context, input *privatezone.BatchDeleteRecordInput, options ...request.Option) (*privatezone.BatchDeleteRecordOutput, error) {
	if m.BatchDeleteRecordFunc != nil {
		return m.BatchDeleteRecordFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) UpdateRecordWithContext(ctx context.Context, input *privatezone.UpdateRecordInput, options ...request.Option) (*privatezone.UpdateRecordOutput, error) {
	if m.UpdateRecordFunc != nil {
		return m.UpdateRecordFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockClient) DeleteRecordWithContext(ctx context.Context, input *privatezone.DeleteRecordInput, options ...request.Option) (*privatezone.DeleteRecordOutput, error) {
	if m.DeleteRecordFunc != nil {
		return m.DeleteRecordFunc(ctx, input)
	}
	return nil, nil
}

func TestCreatePrivateZoneRecord(t *testing.T) {
	// Create a mock client
	mockClient := &MockClient{}

	// Mock CreateRecord response
	mockResponse := &privatezone.CreateRecordOutput{
		Metadata: &response.ResponseMetadata{},
	}
	mockClient.CreateRecordFunc = func(ctx context.Context, input *privatezone.CreateRecordInput) (*privatezone.CreateRecordOutput, error) {
		// Verify input parameters
		assert.Equal(t, "www", *input.Host)
		assert.Equal(t, "A", *input.Type)
		assert.Equal(t, "1.2.3.4", *input.Value)
		assert.Equal(t, int64(123), *input.ZID)
		assert.Equal(t, int32(60), *input.TTL)
		return mockResponse, nil
	}

	// Create PrivateZoneWrapper and inject mock client
	wrapper := &PrivateZoneWrapper{client: mockClient}

	// Call the method
	err := wrapper.CreatePrivateZoneRecord(context.Background(), 123, "www", "A", "1.2.3.4", 60)

	// Verify results
	assert.NoError(t, err)
}

func TestBatchCreatePrivateZoneRecord(t *testing.T) {
	// Create a mock client
	mockClient := &MockClient{}

	// Mock BatchCreateRecord response
	mockResponse := &privatezone.BatchCreateRecordOutput{
		Metadata:  &response.ResponseMetadata{},
		RecordIDs: []*string{volcengine.String("record-1"), volcengine.String("record-2")},
	}
	mockClient.BatchCreateRecordFunc = func(ctx context.Context, input *privatezone.BatchCreateRecordInput) (*privatezone.BatchCreateRecordOutput, error) {
		// Verify input parameters
		assert.Equal(t, int64(123), *input.ZID)
		assert.Len(t, input.Records, 2)
		assert.Equal(t, "www", *input.Records[0].Host)
		assert.Equal(t, "A", *input.Records[0].Type)
		assert.Equal(t, "1.2.3.4", *input.Records[0].Value)
		assert.Equal(t, int32(60), *input.Records[0].TTL)
		assert.Equal(t, "api", *input.Records[1].Host)
		assert.Equal(t, "A", *input.Records[1].Type)
		assert.Equal(t, "5.6.7.8", *input.Records[1].Value)
		assert.Equal(t, int32(60), *input.Records[1].TTL)
		return mockResponse, nil
	}

	// Create PrivateZoneWrapper and inject mock client
	wrapper := &PrivateZoneWrapper{client: mockClient}

	// Prepare test data
	records := []*privatezone.RecordForBatchCreateRecordInput{
		{
			Host:  volcengine.String("www"),
			Type:  volcengine.String("A"),
			Value: volcengine.String("1.2.3.4"),
			TTL:   volcengine.Int32(60),
		},
		{
			Host:  volcengine.String("api"),
			Type:  volcengine.String("A"),
			Value: volcengine.String("5.6.7.8"),
			TTL:   volcengine.Int32(60),
		},
	}

	// Call the method
	err := wrapper.BatchCreatePrivateZoneRecord(context.Background(), 123, records)

	// Verify results
	assert.NoError(t, err)
}

func TestDeletePrivateZoneRecord(t *testing.T) {
	// Create a mock client
	mockClient := &MockClient{}

	// Mock ListRecords response
	mockRecords := &privatezone.ListRecordsOutput{
		Records: []*privatezone.RecordForListRecordsOutput{&privatezone.RecordForListRecordsOutput{
			Host:     volcengine.String("www"),
			Type:     volcengine.String("A"),
			Value:    volcengine.String("1.2.3.4"),
			RecordID: volcengine.String("record-1"),
		}},
		Metadata: &response.ResponseMetadata{},
		Total:    volcengine.Int32(1),
	}
	mockClient.ListRecordsFunc = func(ctx context.Context, input *privatezone.ListRecordsInput) (*privatezone.ListRecordsOutput, error) {
		return mockRecords, nil
	}

	// Mock BatchDeleteRecord response
	mockDeleteResponse := &privatezone.BatchDeleteRecordOutput{
		Metadata: &response.ResponseMetadata{},
	}
	mockClient.BatchDeleteRecordFunc = func(ctx context.Context, input *privatezone.BatchDeleteRecordInput) (*privatezone.BatchDeleteRecordOutput, error) {
		// Verify input parameters
		assert.Equal(t, int64(123), *input.ZID)
		assert.Len(t, input.RecordIDs, 1)
		assert.Equal(t, "record-1", *input.RecordIDs[0])
		return mockDeleteResponse, nil
	}

	// 创建 PrivateZoneWrapper 并注入模拟客户端
	wrapper := &PrivateZoneWrapper{client: mockClient}

	// 调用方法
	err := wrapper.DeletePrivateZoneRecord(context.Background(), 123, "www", "A", []string{"1.2.3.4"})

	// 验证结果
	assert.NoError(t, err)
}
