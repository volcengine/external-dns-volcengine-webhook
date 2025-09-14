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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCleanCNAMEValue(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
	}{{
		name:     "value without trailing dot",
		value:    "example.com",
		expected: "example.com",
	}, {
		name:     "value with trailing dot",
		value:    "example.com.",
		expected: "example.com",
	}, {
		name:     "empty string",
		value:    "",
		expected: "",
	}, {
		name:     "only dot",
		value:    ".",
		expected: "",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := cleanCNAMEValue(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMaskSecret(t *testing.T) {
	cases := []struct {
		name     string
		secret   string
		expected string
	}{{
		name:     "short secret",
		secret:   "123456",
		expected: "****",
	}, {
		name:     "exactly 8 chars",
		secret:   "12345678",
		expected: "****",
	}, {
		name:     "longer than 8 chars",
		secret:   "abcdefghijklmnop",
		expected: "abcd********mnop",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := MaskSecret(tc.secret)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBatchForEach(t *testing.T) {
	cases := []struct {
		name      string
		items     []int
		batchSize int
		expected  []int
		wantErr   bool
	}{{
		name:      "normal case with batch size 2",
		items:     []int{1, 2, 3, 4, 5},
		batchSize: 2,
		expected:  []int{2, 4, 6, 8, 10},
		wantErr:   false,
	}, {
		name:      "empty items",
		items:     []int{},
		batchSize: 2,
		expected:  []int{},
		wantErr:   false,
	}, {
		name:      "batch size 0",
		items:     []int{1, 2, 3},
		batchSize: 0,
		expected:  nil,
		wantErr:   true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Define a function that multiplies each number by 2
			doubleFunc := func(batch []int) ([]int, error) {
				result := make([]int, 0, len(batch))
				for _, item := range batch {
					result = append(result, item*2)
				}
				return result, nil
			}

			result, err := BatchForEach(tc.items, tc.batchSize, doubleFunc)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestQueryAll(t *testing.T) {
	// Mock a query function that returns paginated data
	mockQuery := func(pageNum, pageSize int) ([]string, int, error) {
		// Mock a total of 100 data items
		total := 100
		start := (pageNum - 1) * pageSize
		end := start + pageSize
		if end > total {
			end = total
		}

		// Generate mock data
		data := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			data = append(data, fmt.Sprintf("item-%d", i))
		}

		return data, total, nil
	}

	// Test normal case
	result, err := QueryAll(20, mockQuery)
	assert.NoError(t, err)
	assert.Len(t, result, 100)

	// Test case when pageSize is 0
	result, err = QueryAll(0, mockQuery)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEscapeTXTRecordValue(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
	}{{
		name:     "heritage txt record",
		value:    `"heritage=external-dns,external-dns/owner=example"`,
		expected: "heritage=external-dns,external-dns/owner=example",
	}, {
		name:     "normal txt record",
		value:    `"normal txt record"`,
		expected: `"normal txt record"`,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeTXTRecordValue(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestUnescapeTXTRecordValue(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
	}{{
		name:     "heritage txt record",
		value:    "heritage=external-dns,external-dns/owner=example",
		expected: `"heritage=external-dns,external-dns/owner=example"`,
	}, {
		name:     "normal txt record",
		value:    "normal txt record",
		expected: "normal txt record",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := unescapeTXTRecordValue(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetDNSName(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		domain   string
		expected string
	}{{
		name:     "normal host",
		host:     "www",
		domain:   "example.com",
		expected: "www.example.com",
	}, {
		name:     "null host",
		host:     nullHostPrivateZone,
		domain:   "example.com",
		expected: "example.com",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := getDNSName(tc.host, tc.domain)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSplitDNSName(t *testing.T) {
	cases := []struct {
		name      string
		dnsName   string
		zoneName  string
		expHost   string
		expDomain string
	}{{
		name:      "normal dns name",
		dnsName:   "www.example.com",
		zoneName:  "example.com",
		expHost:   "www",
		expDomain: "example.com",
	}, {
		name:      "dns name with trailing dot",
		dnsName:   "www.example.com.",
		zoneName:  "example.com",
		expHost:   "www",
		expDomain: "example.com",
	}, {
		name:      "root domain",
		dnsName:   "example.com",
		zoneName:  "example.com",
		expHost:   nullHostPrivateZone,
		expDomain: "example.com",
	}, {
		name:      "different domain",
		dnsName:   "www.different.com",
		zoneName:  "example.com",
		expHost:   nullHostPrivateZone,
		expDomain: "",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, domain := splitDNSName(tc.dnsName, tc.zoneName)
			assert.Equal(t, tc.expHost, host)
			assert.Equal(t, tc.expDomain, domain)
		})
	}
}

func TestLoggerAdapter(t *testing.T) {
	// Simple test to ensure LoggerAdapter creation and Log method don't crash
	logger := logrus.NewEntry(logrus.New())
	adapter := NewLoggerAdapter(logger)
	adapter.Log("test log message")

	// Since the Log method outputs based on log level, we can't directly verify the output, but at least ensure the call doesn't crash
	assert.NotNil(t, adapter)
}
