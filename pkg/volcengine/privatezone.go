package volcengine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/volcengine/volc-sdk-golang/service/privatezone"
)

var (
	DefaultPagesize = "100"
)

type privateZoneAPI interface {
	GetPrivateZoneInfo(ctx context.Context, zoneID int64) (privatezone.QueryPrivateZoneResponse, error)
	GetPrivateZones(ctx context.Context) ([]privatezone.TopPrivateZoneResponse, error)
	DeletePrivateZoneRecord(ctx context.Context, zoneID int64, domain string) error
	UpdatePrivateZoneRecord(ctx context.Context, zoneID int64, domain string, target []string) error
	GetPrivateZoneRecords(ctx context.Context, zid string) ([]privatezone.Record, error)
	CreatePrivateZoneRecord(ctx context.Context, zoneID int64, domain, recordType, target string) error
}

type PrivateZoneWrapper struct {
	client *privatezone.Client
}

func (w *PrivateZoneWrapper) ListRecords(ctx context.Context, data privatezone.ListRecordsRequest) (*privatezone.ListRecordsResponse, error) {
	return w.client.ListRecords(ctx, &data)
}

func NewPrivateZoneWrapper() *PrivateZoneWrapper {
	return &PrivateZoneWrapper{
		client: privatezone.InitDNSVolcClient(),
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
	log.Debugf("create record host:%s zone:%d", domain, zoneID)
	if err != nil {
		log.Errorf("Failed to create volcengine record: %v", err)
		return err
	}
	log.Infof("Successfully created volcengine record: %v", res)
	return nil
}

func (w *PrivateZoneWrapper) DeletePrivateZoneRecord(ctx context.Context, zoneID int64, domain string) error {

	records, err := w.GetPrivateZoneRecords(ctx, strconv.FormatInt(zoneID, 10))
	if err != nil {
		return err
	}
	for _, record := range records {
		// todo: add more case
		if strings.HasPrefix(domain, *record.Host) {
			if err := w.client.DeleteRecord(ctx, &privatezone.DeleteRecordRequest{
				RecordID: record.RecordID,
			}); err != nil {
				log.Errorf("Failed to delete volcengine record: %v", err)
				return err
			}
			log.Infof("Successfully deleted volcengine record: %v", record)
		}
	}
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

	if err := w.client.BatchUpdateRecord(ctx, &privatezone.BatchUpdateRecordRequest{
		ZID:     &zoneID,
		Records: rr,
	}); err != nil {
		log.Errorf("Failed to update volcengine record: %v", err)
		return err
	}
	log.Infof("Successfully updated volcengine record: %v", rr)

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
			log.Errorf("Failed to list volcengine records: %v", err)
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
		log.Errorf("Failed to query volcengine privatezone: %v", err)
		return privatezone.QueryPrivateZoneResponse{}, err
	}
	return *zone, nil
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
			log.Errorf("Failed to list volcengine privatezones: %v", err)
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
