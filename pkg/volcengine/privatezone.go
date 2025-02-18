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
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/volcengine/volc-sdk-golang/service/privatezone"
)

var (
	DefaultPagesize = "100"
)

type privateZoneAPI interface {
	GetPrivateZoneInfo(ctx context.Context, zoneID int64) (privatezone.QueryPrivateZoneResponse, error)
	GetPrivateZones(ctx context.Context) ([]privatezone.TopPrivateZoneResponse, error)
	ListPrivateZones(ctx context.Context, vpcID string) ([]privatezone.TopPrivateZoneResponse, error)
	DeletePrivateZoneRecord(ctx context.Context, zoneID int64, domain string) error
	UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, domain string, target []string) error
	GetPrivateZoneRecords(ctx context.Context, zid string) ([]privatezone.Record, error)
	CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string) error
	BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []privatezone.CRecord) error
}

// PrivateZoneWrapper is a wrapper for the privatezone API.
type PrivateZoneWrapper struct {
	// The client for the privatezone API.
	client *privatezone.Client
}

func (w *PrivateZoneWrapper) ListRecords(ctx context.Context, data privatezone.ListRecordsRequest) (*privatezone.ListRecordsResponse, error) {
	return w.client.ListRecords(ctx, &data)
}

// NewPrivateZoneWrapper creates a new PrivateZone wrapper.
func NewPrivateZoneWrapper(regionID, pvzEndpoint string) *PrivateZoneWrapper {
	vc := privatezone.NewVolcCaller()
	vc.Volc.SetAccessKey(os.Getenv("VOLC_ACCESSKEY"))
	vc.Volc.SetSecretKey(os.Getenv("VOLC_SECRETKEY"))
	vc.SetHost(pvzEndpoint)
	vc.SetRegion(regionID)
	vc.Volc.SetScheme("https")
	logrus.Debugf("pvz: region:%s, endpoint:%s, ak:%s, sk:%s", regionID, pvzEndpoint, os.Getenv("VOLC_ACCESSKEY"), os.Getenv("VOLC_SECRETKEY"))
	return &PrivateZoneWrapper{
		client: privatezone.NewClient(vc),
	}
}

func (w *PrivateZoneWrapper) CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string) error {
	res, err := w.client.CreateRecord(ctx, &privatezone.CreateRecordRequest{
		Host:  &domain,
		Line:  &recordType,
		Type:  &recordType,
		Value: &target,
		ZID:   &zoneID,
	})
	logrus.Debugf("create record host:%s zone:%d", domain, zoneID)
	if err != nil {
		logrus.Errorf("Failed to create volcengine record: %v", err)
		return err
	}
	logrus.Infof("Successfully created volcengine record: %v", res)
	return nil
}

func (w *PrivateZoneWrapper) BatchCreatePrivateZoneRecord(ctx context.Context, zoneID int64, records []privatezone.CRecord) error {
	req := &privatezone.BatchCreateRecordRequest{
		Records: records,
		ZID:     &zoneID,
	}
	reqs, err := json.Marshal(req)
	if err != nil {
		logrus.Errorf("Failed to marshal batch create record req: %v", err)
		return err
	}
	logrus.Debugf("batch create record req: %s", string(reqs))
	res, err := w.client.BatchCreateRecord(ctx, req)
	if err != nil {
		logrus.Errorf("Failed to batch create volcengine record: %v", err)
		return err
	}
	resStr, err := json.Marshal(res)
	if err != nil {
		logrus.Errorf("Failed to marshal batch create record res: %v", err)
		return err
	}
	logrus.Infof("Successfully batch created volcengine record: %s", string(resStr))
	return nil
}

func (w *PrivateZoneWrapper) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, domain string) error {

	records, err := w.GetPrivateZoneRecords(ctx, strconv.FormatInt(zoneID, 10))
	if err != nil {
		return err
	}
	recordIDs := make([]string, 0)
	for _, record := range records {
		// todo: add more case
		if strings.HasPrefix(domain, *record.Host) {
			// if err := w.client.DeleteRecord(ctx, &privatezone.DeleteRecordRequest{
			// 	RecordID: record.RecordID,
			// }); err != nil {
			// 	log.Errorf("Failed to delete volcengine record: %v", err)
			// 	return err
			// }
			// log.Infof("Successfully deleted volcengine record: %v", record)
			recordIDs = append(recordIDs, *record.RecordID)
		}
	}
	if len(recordIDs) == 0 {
		logrus.Infof("No record to delete")
		return nil
	}
	return w.batchDeletePrivateZoneRecord(ctx, zoneID, recordIDs)
}

func (w *PrivateZoneWrapper) batchDeletePrivateZoneRecord(ctx context.Context, zoneID int64, recordIDs []string) error {
	req := &privatezone.BatchDeleteRecordRequest{
		RecordIDs: recordIDs,
		ZID:       &zoneID,
	}
	reqs, err := json.Marshal(req)
	if err != nil {
		logrus.Errorf("Failed to marshal batch delete record req: %v", err)
		return err
	}
	logrus.Debugf("batch delete record req: %s", string(reqs))
	err = w.client.BatchDeleteRecord(ctx, req)
	if err != nil {
		logrus.Errorf("Failed to batch delete volcengine record: %v", err)
		return err
	}
	logrus.Infof("Successfully batch deleted volcengine record")
	return nil
}

// UpdatePrivateZoneRecord only support A record type
func (w *PrivateZoneWrapper) UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, domain string, target []string) error {
	records, err := w.GetPrivateZoneRecords(ctx, strconv.FormatInt(zoneID, 10))
	if err != nil {
		return err
	}
	rr := []privatezone.URecord{}
	for _, record := range records {
		// todo: add more case
		if strings.HasPrefix(domain, *record.Host) {
			rr = append(rr, privatezone.URecord{
				RecordID: record.RecordID,
				Host:     &domain,
				Value:    record.Value,
				Type:     record.Type,
				TTL:      record.TTL,
			})
		}
	}
	req := &privatezone.BatchUpdateRecordRequest{
		ZID:     &zoneID,
		Records: rr,
	}
	reqs, err := json.Marshal(req)
	if err != nil {
		logrus.Errorf("Failed to marshal batch update record req: %v", err)
		return err
	}
	logrus.Debugf("batch update record req: %s", string(reqs))
	if err := w.client.BatchUpdateRecord(ctx, req); err != nil {
		logrus.Errorf("Failed to update volcengine record: %v", err)
		return err
	}
	logrus.Infof("Successfully updated volcengine record")

	return nil
}

func (w *PrivateZoneWrapper) GetPrivateZoneRecords(ctx context.Context, zid string) ([]privatezone.Record, error) {
	var res []privatezone.Record
	pageNumber := fmt.Sprint(1)
	for {
		req := privatezone.ListRecordsRequest{
			ZID:        &zid,
			PageSize:   &DefaultPagesize,
			PageNumber: &pageNumber,
		}
		records, err := w.client.ListRecords(ctx, &req)
		if err != nil {
			logrus.Errorf("Failed to list volcengine records: %v", err)
			return nil, err
		}

		for _, record := range records.Records {
			res = append(res, record)
		}
		nextPage := getNextPageNumber(*records.PageNumber, *records.PageSize, *records.Total)
		if nextPage == 0 {
			break
		}
		pageNumber = strconv.Itoa(int(nextPage))
	}

	return res, nil
}

func (w *PrivateZoneWrapper) GetPrivateZoneInfo(ctx context.Context, zoneID int64) (privatezone.QueryPrivateZoneResponse, error) {
	zoneIDStr := strconv.FormatInt(zoneID, 10)
	zone, err := w.client.QueryPrivateZone(ctx, &privatezone.QueryPrivateZoneRequest{
		ZID: &zoneIDStr,
	})
	if err != nil {
		logrus.Errorf("Failed to query volcengine privatezone: %v", err)
		return privatezone.QueryPrivateZoneResponse{}, err
	}
	return *zone, nil
}

func (w *PrivateZoneWrapper) ListPrivateZones(ctx context.Context, vpcID string) ([]privatezone.TopPrivateZoneResponse, error) {
	var res []privatezone.TopPrivateZoneResponse
	pageNumber := fmt.Sprint(1)
	for {
		zones, err := w.client.ListPrivateZones(ctx, &privatezone.ListPrivateZonesRequest{
			PageSize:   &DefaultPagesize,
			PageNumber: &pageNumber,
			VpcID:      &vpcID,
		})
		if err != nil {
			logrus.Errorf("Failed to list volcengine privatezones: %v", err)
			return nil, err
		}
		res = append(res, zones.Zones...)
		nextPage := getNextPageNumber(*zones.PageNumber, *zones.PageSize, *zones.Total)
		if nextPage == 0 {
			break
		}
		pageNumber = strconv.Itoa(int(nextPage))
	}
	return res, nil
}

func (w *PrivateZoneWrapper) GetPrivateZones(ctx context.Context) ([]privatezone.TopPrivateZoneResponse, error) {
	var res []privatezone.TopPrivateZoneResponse

	pageNumber := fmt.Sprint(1)
	for {
		zones, err := w.client.ListPrivateZones(ctx, &privatezone.ListPrivateZonesRequest{
			PageSize:   &DefaultPagesize,
			PageNumber: &pageNumber,
		})
		if err != nil {
			logrus.Errorf("Failed to list volcengine privatezones: %v", err)
			return nil, err
		}
		for _, zone := range zones.Zones {
			res = append(res, zone)
		}
		nextPage := getNextPageNumber(*zones.PageNumber, *zones.PageSize, *zones.Total)
		if nextPage == 0 {
			break
		}
		pageNumber = strconv.Itoa(int(nextPage))
	}

	return res, nil
}

func getNextPageNumber(pageNumber, pageSize, totalCount int64) int64 {
	if pageNumber*pageSize >= totalCount {
		return 0
	}
	return pageNumber + 1
}

// compareRecords compare dnsname and host+zonename, return true if equal
func compareRecords(dnsname string, zonename string, host string) bool {
	return strings.Compare(dnsname, fmt.Sprintf("%s.%s", host, zonename)) == 0
}
