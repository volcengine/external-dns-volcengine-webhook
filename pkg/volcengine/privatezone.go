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
	"sigs.k8s.io/external-dns/endpoint"
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
	ListRecordsByVPC(ctx context.Context, vpc string) (endpoints []*endpoint.Endpoint, err error)
	GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error)
	CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string, TTL int32) error
	BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error
	DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host string, recordType string, targets []string) error
}

var _ privateZoneAPI = &PrivateZoneWrapper{}

// privateZoneClient is an interface that contains only the methods actually used by PrivateZoneWrapper
type privateZoneClient interface {
	ListPrivateZonesWithContext(ctx context.Context, input *privatezone.ListPrivateZonesInput, options ...request.Option) (*privatezone.ListPrivateZonesOutput, error)
	ListRecordsWithContext(ctx context.Context, input *privatezone.ListRecordsInput, options ...request.Option) (*privatezone.ListRecordsOutput, error)
	CreateRecordWithContext(ctx context.Context, input *privatezone.CreateRecordInput, options ...request.Option) (*privatezone.CreateRecordOutput, error)
	BatchCreateRecordWithContext(ctx context.Context, input *privatezone.BatchCreateRecordInput, options ...request.Option) (*privatezone.BatchCreateRecordOutput, error)
	BatchDeleteRecordWithContext(ctx context.Context, input *privatezone.BatchDeleteRecordInput, options ...request.Option) (*privatezone.BatchDeleteRecordOutput, error)
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

// ListRecordsByVPC returns the list of private zones for the given VPC.
func (w *PrivateZoneWrapper) ListRecordsByVPC(ctx context.Context, vpc string) (endpoints []*endpoint.Endpoint, err error) {
	// step 1: get all private zones bind to vpc
	vpcZones, err := w.ListPrivateZones(ctx, vpc)
	if err != nil {
		logrus.Errorf("Failed to list volcengine privatezones: %v", err)
		return nil, err
	}

	// step 2: get all record with private zone
	for _, zone := range vpcZones {
		records, err := w.GetPrivateZoneRecords(ctx, int64(volcengine.Int32Value(zone.ZID)))
		if err != nil {
			logrus.Errorf("Failed to get privatezone records: %v", err)
			return nil, err
		}

		if len(records) == 0 {
			continue
		}
		// step 3: convert record to endpoint, merge targets with same host and type
		recordsMap := w.groupPrivateZoneRecords(records)
		for _, recordList := range recordsMap {
			record := recordList[0]
			dnsName := getDNSName(record.Host, *zone.ZoneName)
			ttl := record.TTL
			targets := make([]string, 0)
			for _, r := range recordList {
				target := r.Target
				//if record.Type == "TXT" {
				//	target = unescapeTXTRecordValue(target)
				//	logrus.Debugf("Unescaped TXT record target: (%s)", target)
				//}
				targets = append(targets, target)
			}
			// Domain: record.Host + "." + zoneInfo.ZoneName
			// Type:  record.Type
			// Target: record.Value
			// TTL: record.TTL
			endpoints = append(endpoints, endpoint.NewEndpointWithTTL(dnsName, record.Type, endpoint.TTL(ttl), targets...))
		}
	}

	logrus.Debugf("Returned Volcengine Private Zone records: %+v", endpoints)
	return endpoints, nil
}

func (w *PrivateZoneWrapper) groupPrivateZoneRecords(zone []*privatezone.RecordForListRecordsOutput) (endpointMap map[string][]Record) {
	endpointMap = make(map[string][]Record)

	for _, record := range zone {
		key := volcengine.StringValue(record.Type) + ":" + volcengine.StringValue(record.Host)
		recordList := endpointMap[key]
		endpointMap[key] = append(recordList, Record{
			Host:   volcengine.StringValue(record.Host),
			Type:   volcengine.StringValue(record.Type),
			TTL:    int(volcengine.Int32Value(record.TTL)),
			Target: volcengine.StringValue(record.Value),
		})
	}

	return endpointMap
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

// DeletePrivateZoneRecord deletes a private zone record.
// multiple targets will to delete multiple records with same value
func (w *PrivateZoneWrapper) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType string, targets []string) error {
	records, err := w.GetPrivateZoneRecords(ctx, zoneID)
	if err != nil {
		return err
	}
	recordIDs := make([]string, 0)
	for _, record := range records {
		if host == volcengine.StringValue(record.Host) &&
			recordType == volcengine.StringValue(record.Type) {
			value := volcengine.StringValue(record.Value)
			if volcengine.StringValue(record.Type) == "TXT" {
				value = unescapeTXTRecordValue(value)
				logrus.Tracef("Unescape txt record value: (%s), host: %s, zid: %d", value, host, zoneID)
			}
			for _, target := range targets {
				if target == value {
					recordIDs = append(recordIDs, volcengine.StringValue(record.RecordID))
					break
				}
			}
		}
	}
	if len(recordIDs) == 0 {
		logrus.Infof("No record to delete")
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
