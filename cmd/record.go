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
	"os"
	"strconv"
	"strings"

	"volcengine-provider/pkg/volcengine"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	recordCmd = &cobra.Command{
		Use:   "record",
		Short: "Add/Delete/List records",
	}
	recordAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add record",
		Run: func(cmd *cobra.Command, args []string) {
			initSDK()
			recordAddHandler()
		},
	}
	recordDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete record",
		Run: func(cmd *cobra.Command, args []string) {
			initSDK()
			recordDelHandler()
		},
	}
	recordListCmd = &cobra.Command{
		Use:   "list",
		Short: "List record",
		Run: func(cmd *cobra.Command, args []string) {
			initSDK()
			client := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("endpoint"))
			if err := listRecord(client, zone); err != nil {
				log.Errorf("Failed to show record: %v", err)
				return
			}
		},
	}

	record string
	host   string
	zone   int64
)

func init() {
	rootCmd.AddCommand(recordCmd)
	recordCmd.AddCommand(recordAddCmd)
	recordCmd.AddCommand(recordDeleteCmd)
	recordCmd.AddCommand(recordListCmd)

	recordCmd.PersistentFlags().Int64Var(&zone, "zone", 0, "zone id")
	recordAddCmd.PersistentFlags().StringVar(&record, "record", "", "record, like host,type,target")
	recordDeleteCmd.PersistentFlags().StringVar(&host, "host", "", "record host")
}

func initSDK() {
	accessKey := viper.GetString("access_key")
	accessSecret := viper.GetString("access_secret")
	os.Setenv("VOLC_SECRETKEY", accessSecret) // 为 sdk 设置环境变量
	os.Setenv("VOLC_ACCESSKEY", accessKey)
}

func recordAddHandler() {
	client := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("endpoint"))
	recordValue := strings.Split(record, ",")
	if len(recordValue) != 3 {
		log.Errorf("Invalid record value: %s", record)
		return
	}
	if err := addRecord(client, recordValue[0], recordValue[1], recordValue[2]); err != nil {
		log.Errorf("Add record error: %v", err)
		return
	}
}

func recordDelHandler() {
	client := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("endpoint"))
	if err := delRecord(client, host); err != nil {
		log.Errorf("Delete record error: %v", err)
		return
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

func listRecord(client *volcengine.PrivateZoneWrapper, zoneID int64) error {
	log.Debugf("list record: %d", zoneID)
	records, err := client.GetPrivateZoneRecords(context.Background(), strconv.FormatInt(zoneID, 10))
	if err != nil {
		log.Errorf("Failed to show record: %v", err)
		return err
	}
	for _, r := range records {
		if r.Host != nil {
			log.Infof("host: %s, type: %s, target: %s, ttl: %d", *r.Host, *r.Type, *r.Value, *r.TTL)
		}
	}
	return nil
}
