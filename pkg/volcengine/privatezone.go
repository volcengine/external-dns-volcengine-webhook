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
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volcengine-go-sdk/service/privatezone"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
	"sigs.k8s.io/external-dns/endpoint"
)

var (
	DefaultPageSize  = 100
	DefaultBatchSize = 100
)

type Record struct {
	Host   string `json:"host"`
	Type   string `json:"type"`
	TTL    int    `json:"ttl"`
	Target string `json:"target"`
}

type privateZoneAPI interface {
	GetPrivateZoneInfo(ctx context.Context, zoneID int64) (*privatezone.QueryPrivateZoneOutput, error)
	ListPrivateZones(ctx context.Context, vpcID string) ([]*privatezone.ZoneForListPrivateZonesOutput, error)
	ListRecordsByVPC(ctx context.Context, vpc string) (endpoints []*endpoint.Endpoint, err error)
	GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error)
	CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string, TTL int32) error
	BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error
	DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host string, recordType string, targets []string) error
}

var _ privateZoneAPI = &PrivateZoneWrapper{}

// PrivateZoneWrapper is a wrapper for the privatezone API.
type PrivateZoneWrapper struct {
	// The client for the privatezone API.
	client privatezone.PRIVATEZONEAPI
}

// NewPrivateZoneWrapper creates a new PrivateZone wrapper.
func NewPrivateZoneWrapper(regionID, pvzEndpoint string, credentials *credentials.Credentials) (*PrivateZoneWrapper, error) {
	c := volcengine.NewConfig().
		WithRegion(regionID).
		WithCredentials(credentials).
		WithEndpoint(pvzEndpoint).
		WithDisableSSL(true).
		WithLogLevel(volcengine.LogLevelType(logrus.GetLevel()))
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

func (w *PrivateZoneWrapper) CreatePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType, target string, TTL int32) error {
	request := &privatezone.CreateRecordInput{
		Host:   &host,
		Type:   &recordType,
		Value:  &target,
		ZID:    &zoneID,
		TTL:    &TTL,
		Remark: volcengine.String("managed by external-dns"),
	}
	resp, err := w.client.CreateRecordWithContext(ctx, request)
	logrus.Tracef("create record request: %+v, resp: %+v", request, resp)
	if err != nil {
		logrus.Errorf("Failed to create volcengine record: %v", err)
		return err
	}
	if resp.Metadata.Error != nil {
		logrus.Errorf("Failed to create volcengine record: %v", resp.Metadata.Error)
		return err
	}

	logrus.Infof("Successfully created volcengine record: %+v", resp)
	return nil
}

// BatchCreatePrivateZoneRecord creates a batch of private zone records.
// same host and type will be merged into one record with multiple values in privatezone.
//   - TTL will use first record's TTL.
//   - Remark can be set in every record.
func (w *PrivateZoneWrapper) BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []*privatezone.RecordForBatchCreateRecordInput) error {
	req := &privatezone.BatchCreateRecordInput{
		Records: records,
		ZID:     &zoneID,
	}
	reqs, err := json.Marshal(req)
	if err != nil {
		logrus.Errorf("Failed to marshal batch create record req: %v", err)
		return err
	}

	resp, err := w.client.BatchCreateRecordWithContext(ctx, req)
	logrus.Tracef("batch create record req: %s, resp: %s", string(reqs), resp)
	if err != nil {
		logrus.Errorf("Failed to batch create volcengine record: %v", err)
		return err
	}
	if resp.Metadata.Error != nil {
		logrus.Errorf("Failed to batch create volcengine record: %v", resp.Metadata.Error)
		return err
	}

	logrus.Infof("Successfully batch created volcengine record: %+v", resp)
	return nil
}

func (w *PrivateZoneWrapper) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, host, recordType string, targets []string) error {
	records, err := w.GetPrivateZoneRecords(ctx, zoneID)
	if err != nil {
		return err
	}
	recordIDs := make([]string, 0)
	for _, record := range records {
		if host == volcengine.StringValue(record.Host) && recordType == volcengine.StringValue(record.Type) {
			value := volcengine.StringValue(record.Value)
			if volcengine.StringValue(record.Type) == "TXT" {
				value = unescapeTXTRecordValue(value)
				logrus.Debugf("Unescape txt record value: (%s), host: %s, zid: %d", value, host, zoneID)
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
	_, err := BatchForEach(recordIDs, DefaultBatchSize, func(ids []string) ([]string, error) {
		req := &privatezone.BatchDeleteRecordInput{
			RecordIDs: volcengine.StringSlice(ids),
			ZID:       &zoneID,
		}
		resp, err := w.client.BatchDeleteRecordWithContext(ctx, req)
		logrus.Debugf("Batch delete record req: %s, resp: %s", req, resp)
		if err != nil {
			logrus.Errorf("Failed to batch delete volcengine record: %v", err)
			return nil, err
		}
		if resp.Metadata.Error != nil {
			logrus.Errorf("Failed to batch delete volcengine record: %v", resp.Metadata.Error)
			return nil, err
		}

		return ids, nil
	})
	if err != nil {
		logrus.Errorf("Failed to batch delete volcengine record: %v", err)
		return err
	}

	logrus.Infof("Successfully batch deleted volcengine record, %v", recordIDs)
	return nil
}

func (w *PrivateZoneWrapper) GetPrivateZoneRecords(ctx context.Context, zid int64) ([]*privatezone.RecordForListRecordsOutput, error) {
	res, err := QueryAll(DefaultPageSize, func(pageNum, pageSize int) ([]*privatezone.RecordForListRecordsOutput, int, error) {
		req := privatezone.ListRecordsInput{
			ZID:        &zid,
			PageSize:   volcengine.String(strconv.FormatInt(int64(pageSize), 10)),
			PageNumber: volcengine.Int32(int32(pageNum)),
		}
		resp, err := w.client.ListRecordsWithContext(ctx, &req)
		logrus.Tracef("List records: req: %s, resp: %s", req, resp)
		if err != nil {
			logrus.Errorf("Failed to list volcengine records: %v", err)
			return nil, 0, err
		}
		if resp.Metadata.Error != nil {
			logrus.Errorf("Failed to list volcengine records: %v", resp.Metadata.Error)
			return nil, 0, err
		}
		return resp.Records, int(volcengine.Int32Value(resp.Total)), nil
	})
	if err != nil {
		logrus.Errorf("Failed to list volcengine records: %v", err)
		return nil, err
	}

	logrus.Debugf("Successfully list volcengine records: %+v", res)
	return res, nil
}

func (w *PrivateZoneWrapper) GetPrivateZoneInfo(ctx context.Context, zoneID int64) (*privatezone.QueryPrivateZoneOutput, error) {
	resp, err := w.client.QueryPrivateZoneWithContext(ctx, &privatezone.QueryPrivateZoneInput{
		ZID: &zoneID,
	})
	logrus.Tracef("Get private zone info zid: %d, resp: %+v", zoneID, resp)
	if err != nil {
		logrus.Errorf("Failed to query volcengine privatezone: %v", err)
		return nil, err
	}
	if resp.Metadata.Error != nil {
		logrus.Errorf("Failed to query volcengine privatezone: %v", resp.Metadata.Error)
		return nil, err
	}

	logrus.Debugf("Successfully query volcengine privatezone: %+v", resp)
	return resp, nil
}

func (w *PrivateZoneWrapper) ListPrivateZones(ctx context.Context, vpcID string) ([]*privatezone.ZoneForListPrivateZonesOutput, error) {
	zones, err := QueryAll(DefaultPageSize, func(pageNum, pageSize int) ([]*privatezone.ZoneForListPrivateZonesOutput, int, error) {
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
		if err != nil {
			logrus.Errorf("Failed to list volcengine privatezones: %v", err)
			return nil, 0, err
		}
		if resp.Metadata.Error != nil {
			logrus.Errorf("Failed to list volcengine privatezones: %v", resp.Metadata.Error)
			return nil, 0, err
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
