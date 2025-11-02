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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/request"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

var (
	defaultPageSize  = 100
	defaultBatchSize = 100
	// null host for private zone
	nullHostPrivateZone = "@"

	defaultRecordRemark = "managed by external-dns"
)

type Record struct {
	Host   string `json:"host"`
	Type   string `json:"type"`
	TTL    int    `json:"ttl"`
	Target string `json:"target"`
}

type privateZoneAPI interface {
	ListPrivateZones(ctx context.Context, vpcID string) ([]*privatezone.ZoneForListPrivateZonesOutput, error)
	GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error)
	CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string, TTL int32) error
	BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error
	UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, recordID string, host, recordType, target string, TTL int32) error
	DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType string, targets []string) error
	DeletePrivateZoneRecordById(ctx context.Context, zoneID int64, recordID string) error
}

var _ privateZoneAPI = &PrivateZoneWrapper{}

// privateZoneClient is an interface that contains only the methods actually used by PrivateZoneWrapper
type privateZoneClient interface {
	ListPrivateZonesWithContext(ctx context.Context, input *privatezone.ListPrivateZonesInput, options ...request.Option) (*privatezone.ListPrivateZonesOutput, error)
	ListRecordsWithContext(ctx context.Context, input *privatezone.ListRecordsInput, options ...request.Option) (*privatezone.ListRecordsOutput, error)
	CreateRecordWithContext(ctx context.Context, input *privatezone.CreateRecordInput, options ...request.Option) (*privatezone.CreateRecordOutput, error)
	UpdateRecordWithContext(ctx context.Context, input *privatezone.UpdateRecordInput, options ...request.Option) (*privatezone.UpdateRecordOutput, error)
	BatchCreateRecordWithContext(ctx context.Context, input *privatezone.BatchCreateRecordInput, options ...request.Option) (*privatezone.BatchCreateRecordOutput, error)
	BatchDeleteRecordWithContext(ctx context.Context, input *privatezone.BatchDeleteRecordInput, options ...request.Option) (*privatezone.BatchDeleteRecordOutput, error)
	DeleteRecordWithContext(ctx context.Context, input *privatezone.DeleteRecordInput, options ...request.Option) (*privatezone.DeleteRecordOutput, error)
}

// PrivateZoneWrapper is a wrapper for the privatezone API.
type PrivateZoneWrapper struct {
	// The client for the privatezone API.
	client privateZoneClient
}

// NewPrivateZoneWrapper creates a new PrivateZone wrapper.
func NewPrivateZoneWrapper(regionID, pvzEndpoint string, credentials *credentials.Credentials) (*PrivateZoneWrapper, error) {
	c := volcengine.NewConfig().
		WithRegion(regionID).
		WithCredentials(credentials).
		WithEndpoint(pvzEndpoint).
		WithLogger(NewLoggerAdapter(logrus.StandardLogger().WithField("client", "privatezone")))
	s, err := session.NewSession(c)
	if err != nil {
		logrus.Errorf("Failed to create volcengine session: %v", err)
		return nil, err
	}
	pc := privatezone.New(s)

	return &PrivateZoneWrapper{
		client: pc,
	}, nil
}

// CreatePrivateZoneRecord creates a new private zone record.
func (w *PrivateZoneWrapper) CreatePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType, target string, TTL int32) error {
	request := &privatezone.CreateRecordInput{
		Host:   &host,
		Type:   &recordType,
		Value:  &target,
		ZID:    &zoneID,
		TTL:    &TTL,
		Remark: volcengine.String(defaultRecordRemark),
	}
	resp, err := w.client.CreateRecordWithContext(ctx, request)
	logrus.Tracef("Create record request: %+v, resp: %+v", request, resp)
	if err != nil || resp.Metadata.Error != nil {
		return fmt.Errorf("failed to create privatezone record, err: %v, resp: %v", err, resp)
	}

	logrus.Infof("Successfully created volcengine record: %+v", resp)
	return nil
}

// BatchCreatePrivateZoneRecord creates a batch of private zone records.
// same host and type will be merged into one record with multiple values in privatezone.
//   - TTL will use first record's TTL.
//   - Remark can be set in every record.
func (w *PrivateZoneWrapper) BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error {
	_, err := BatchForEach(records, defaultBatchSize, func(partialRecords []*privatezone.RecordForBatchCreateRecordInput) ([]*string, error) {
		req := &privatezone.BatchCreateRecordInput{
			Records: partialRecords,
			ZID:     &zoneID,
		}
		reqs, err := json.Marshal(req)
		if err != nil {
			logrus.Errorf("Failed to marshal batch create record req: %v", err)
			return nil, err
		}

		resp, err := w.client.BatchCreateRecordWithContext(ctx, req)
		logrus.Tracef("Batch create record req: %s, resp: %s", string(reqs), resp)
		if err != nil || resp.Metadata.Error != nil {
			// directly print resp avoid Metadata is nil
			return nil, fmt.Errorf("failed to batch create privatezone record, err: %v, resp: %v", err, resp)
		}

		logrus.Infof("Successfully batch created privatezone record: %s", resp.String())
		return resp.RecordIDs, nil
	})
	if err != nil {
		logrus.Errorf("Failed to batch create privatezone record: %v", err)
		return err
	}

	return nil
}

func (w *PrivateZoneWrapper) UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, recordID string, host, recordType, target string, TTL int32) error {
	req := &privatezone.UpdateRecordInput{
		RecordID: &recordID,
		Host:     &host,
		Type:     &recordType,
		Value:    &target,
		ZID:      &zoneID,
		TTL:      &TTL,
	}
	resp, err := w.client.UpdateRecordWithContext(ctx, req)
	logrus.Tracef("Update record request: %+v, resp: %+v", req, resp)
	if err != nil || resp.Metadata.Error != nil {
		return fmt.Errorf("failed to update privatezone record, err: %v, resp: %v", err, resp)
	}
	logrus.Infof("Successfully updated volcengine record: %+v", resp)
	return nil
}

func (w *PrivateZoneWrapper) DeletePrivateZoneRecordById(ctx context.Context, zoneID int64, recordID string) error {
	req := &privatezone.DeleteRecordInput{
		RecordID: &recordID,
		ZID:      &zoneID,
	}
	resp, err := w.client.DeleteRecordWithContext(ctx, req)
	logrus.Tracef("Delete record request: %+v, resp: %+v", req, resp)
	if err != nil || resp.Metadata.Error != nil {
		return fmt.Errorf("failed to delete privatezone record, err: %v, resp: %v", err, resp)
	}
	logrus.Infof("Successfully deleted volcengine record: %+v", resp)
	return nil
}

// DeletePrivateZoneRecord deletes a private zone record.
// multiple targets will to delete multiple records with same value
func (w *PrivateZoneWrapper) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType string, targets []string) error {
	records, err := w.GetPrivateZoneRecords(ctx, zoneID)
	if err != nil {
		return err
	}
	recordIDs := make([]string, 0)
	found := false
	for _, record := range records {
		if host == volcengine.StringValue(record.Host) &&
			recordType == volcengine.StringValue(record.Type) {
			value := volcengine.StringValue(record.Value)
			if volcengine.StringValue(record.Type) == "TXT" {
				value = unescapeTXTRecordValue(value)
				logrus.Tracef("Unescape txt record value: (%s), host: %s, zid: %d", value, host, zoneID)
			}
			if volcengine.StringValue(record.Type) == "CNAME" {
				value = normalizeDomain(value)
				logrus.Tracef("Clean cname target: (%s), host: %s, zid: %d", value, host, zoneID)
			}

			for _, target := range targets {
				if target == value {
					recordIDs = append(recordIDs, volcengine.StringValue(record.RecordID))
					found = true
					break
				}
			}
			if !found {
				logrus.Debugf("Not found record bacause different value: host: %s, type: %s, value: %s, expectTargets: %v", host, recordType, value, targets)
			}
		}
	}
	if len(recordIDs) == 0 {
		logrus.Errorf("Not found record to delete.  zid: %d, host: %s, recordType %s, targes: %v", zoneID, host, recordType, targets)
		return nil
	}

	return w.batchDeletePrivateZoneRecord(ctx, zoneID, recordIDs)
}

func (w *PrivateZoneWrapper) batchDeletePrivateZoneRecord(ctx context.Context, zoneID int64, recordIDs []string) error {
	_, err := BatchForEach(recordIDs, defaultBatchSize, func(ids []string) ([]string, error) {
		req := &privatezone.BatchDeleteRecordInput{
			RecordIDs: volcengine.StringSlice(ids),
			ZID:       &zoneID,
		}
		resp, err := w.client.BatchDeleteRecordWithContext(ctx, req)
		logrus.Tracef("Batch delete record req: %s, resp: %s", req, resp)
		if err != nil || resp.Metadata.Error != nil {
			return nil, fmt.Errorf("failed to delete privatezone records, err: %v, resp: %v", err, resp)
		}

		return ids, nil
	})
	if err != nil {
		logrus.Errorf("Failed to batch delete privatezone record: %v", err)
		return err
	}

	logrus.Infof("Successfully batch deleted privatezone record, zid: %d, records: %v", zoneID, recordIDs)
	return nil
}

// GetPrivateZoneRecords returns the list of private zone records.
func (w *PrivateZoneWrapper) GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error) {
	res, err := QueryAll(defaultPageSize, func(pageNum, pageSize int) ([]*privatezone.RecordForListRecordsOutput, int, error) {
		req := privatezone.ListRecordsInput{
			ZID:        &zid,
			PageSize:   volcengine.String(strconv.FormatInt(int64(pageSize), 10)),
			PageNumber: volcengine.Int32(int32(pageNum)),
		}
		resp, err := w.client.ListRecordsWithContext(ctx, &req)
		logrus.Tracef("List records req: %s, resp: %+v", req, resp)
		if err != nil || resp.Metadata.Error != nil {
			return nil, 0, fmt.Errorf("failed to list privatezone records, err: %v, resp: %v", err, resp)
		}
		return resp.Records, int(volcengine.Int32Value(resp.Total)), nil
	})
	if err != nil {
		logrus.Errorf("Failed to list privatezone records: %v", err)
		return nil, err
	}

	logrus.Debugf("Successfully list privatezone records: %+v", res)
	return res, nil
}

func (w *PrivateZoneWrapper) ListPrivateZones(ctx context.Context, vpcID string) ([]*privatezone.ZoneForListPrivateZonesOutput, error) {
	zones, err := QueryAll(defaultPageSize, func(pageNum, pageSize int) ([]*privatezone.ZoneForListPrivateZonesOutput, int, error) {
		req := &privatezone.ListPrivateZonesInput{
			PageSize:   volcengine.Int32(int32(pageSize)),
			PageNumber: volcengine.Int32(int32(pageNum)),
			VpcID: func() *string {
				if vpcID != "" {
					return volcengine.String(vpcID)
				}
				return nil
			}(),
		}
		resp, err := w.client.ListPrivateZonesWithContext(ctx, req)
		logrus.Tracef("List volcengine zones: req: %s, resp: %s", req, resp)
		if err != nil || resp.Metadata.Error != nil {
			return nil, 0, fmt.Errorf("failed to list volcengine privatezones, err: %v, resp: %v", err, resp)
		}
		return resp.Zones, int(volcengine.Int32Value(resp.Total)), nil
	})
	if err != nil {
		logrus.Errorf("Failed to list volcengine privatezones: %v", err)
		return nil, err
	}

	logrus.Debugf("Successfully list volcengine privatezones: %+v", zones)
	return zones, nil
}
