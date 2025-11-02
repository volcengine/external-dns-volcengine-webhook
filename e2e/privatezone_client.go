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

package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// PrivateZoneClient encapsulates operations on Volcengine PrivateZone
type PrivateZoneClient struct {
	client privatezone.PRIVATEZONEAPI
}

// NewPrivateZoneClient creates a new PrivateZone client
func NewPrivateZoneClient(config *TestConfig) (*PrivateZoneClient, error) {
	volcConfig, err := CreateVolcengineClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create volcengine config: %w", err)
	}
	s, err := session.NewSession(volcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	client := privatezone.New(s)
	return &PrivateZoneClient{client: client}, nil
}

// ListRecords queries all records of a specified private domain
func (p *PrivateZoneClient) ListRecords(ctx context.Context, zoneID int64) ([]*privatezone.RecordForListRecordsOutput, error) {
	request := &privatezone.ListRecordsInput{
		ZID: &zoneID,
	}

	resp, err := p.client.ListRecordsWithContext(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list private zone records: %w", err)
	}

	return resp.Records, nil
}

// DeleteRecord deletes a specified record
func (p *PrivateZoneClient) DeleteRecord(ctx context.Context, zoneID int64, recordID string) error {
	request := &privatezone.DeleteRecordInput{
		ZID:      &zoneID,
		RecordID: &recordID,
	}

	_, err := p.client.DeleteRecordWithContext(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to delete private zone record: %w", err)
	}

	return nil
}

// CleanupRecordsForDomain deletes all records for a specified domain, used for test cleanup
func (p *PrivateZoneClient) CleanupRecordsForDomain(ctx context.Context, zoneID int64, domain string) error {
	records, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return err
	}

	for _, record := range records {
		if *record.Host == domain {
			if err := p.DeleteRecord(ctx, zoneID, *record.RecordID); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetRecordByHostAndType gets a record by host name and record type
func (p *PrivateZoneClient) GetRecordByHostAndType(ctx context.Context, zoneID int64, host string, recordType string) (*privatezone.RecordForListRecordsOutput, error) {
	records, err := p.ListRecords(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if *record.Host == host && *record.Type == recordType {
			return record, nil
		}
	}

	return nil, fmt.Errorf("record not found: host=%s, type=%s", host, recordType)
}

// WaitForRecordDeleted waits for a record to be deleted
func (p *PrivateZoneClient) WaitForRecordDeleted(ctx context.Context, zoneID int64, host string, recordType string, timeout time.Duration) (bool, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			return false, fmt.Errorf("timeout waiting for record %s to be deleted", host)
		case <-ticker.C:
			_, err := p.GetRecordByHostAndType(ctx, zoneID, host, recordType)
			if err != nil {
				// If record does not exist, consider deletion successful
				if strings.Contains(err.Error(), "record not found") {
					return true, nil
				}
				return false, err
			}
			// If record still exists, continue waiting
		}
	}
}
