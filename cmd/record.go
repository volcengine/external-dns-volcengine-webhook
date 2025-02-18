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

package cmd

import (
	"context"
	"strconv"
	"volcengine-provider/pkg/volcengine"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	recordCmd = &cobra.Command{
		Use:   "record",
		Short: "Add/Delete/List records",
		Run: func(cmd *cobra.Command, args []string) {
			recordHandler()
		},
	}

	addrecord []string
	delrecord []string
	vpc       string
	zone      int64
)

func init() {
	rootCmd.AddCommand(recordCmd)
	recordCmd.PersistentFlags().StringSliceVarP(&addrecord, "add", "a", []string{}, "add record")
	recordCmd.PersistentFlags().StringSliceVarP(&delrecord, "del", "d", []string{}, "del record")
	recordCmd.PersistentFlags().StringVarP(&vpc, "vpc", "v", "", "vpc id")
	recordCmd.PersistentFlags().Int64VarP(&zone, "zone", "z", 321963, "zone id")
}

func recordHandler() {
	client := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("pvz_endpoint"))

	if len(addrecord) > 0 {
		for _, record := range addrecord {
			addRecord(client, record, "A", "1.1.1.1")
		}
	}

	if len(delrecord) > 0 {
		for _, record := range delrecord {
			delRecord(client, record)
		}
	}

	if zone != 0 {
		showRecord(client, zone)
	}
}

func addRecord(client *volcengine.PrivateZoneWrapper, host string, recordType string, target string) error {
	log.Debugf("add record: %s, type: %s, target: %s", host, recordType, target)
	err := client.CreatePrivateZoneRecord(context.Background(), zone, host, recordType, target)
	if err != nil {
		log.Errorf("Failed to add record: %v", err)
		return err
	}
	return nil
}

func delRecord(client *volcengine.PrivateZoneWrapper, host string) error {
	log.Debugf("del record: %s", host)
	err := client.DeletePrivateZoneRecord(context.Background(), zone, host)
	if err != nil {
		log.Errorf("Failed to del record: %v", err)
		return err
	}
	return nil
}

func showRecord(client *volcengine.PrivateZoneWrapper, zoneID int64) error {
	log.Debugf("show record: %d", zoneID)
	records, err := client.GetPrivateZoneRecords(context.Background(), strconv.FormatInt(zoneID, 10))
	if err != nil {
		log.Errorf("Failed to show record: %v", err)
		return err
	}
	for _, record := range records {
		if record.Host != nil {
			log.Infof("host: %s, type: %s, target: %s", *record.Host, *record.Type, *record.Value)
		}

	}
	return nil
}
