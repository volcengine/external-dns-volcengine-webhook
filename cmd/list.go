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
	"fmt"
	"volcengine-provider/pkg/volcengine"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List zone or records",
		Run: func(cmd *cobra.Command, args []string) {
			listZones()
		},
	}

	vpcID string
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.PersistentFlags().StringVar(&vpcID, "vpc", "", "VPC ID")
}

func listZones() {
	client := volcengine.NewPrivateZoneWrapper(viper.GetString("region"), viper.GetString("pvz_endpoint"))

	// zones, err := client.GetPrivateZones(context.Background())
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }
	// for _, zone := range zones {
	// 	fmt.Printf("Zone ID: %d, Zone Name: %s\n", *zone.ZID, *zone.ZoneName)
	// }

	endpoints, err := volcengine.Records(context.Background(), client, vpcID)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, ep := range endpoints {
		fmt.Printf("dns:%s endpoints:%s\n", ep.DNSName, ep.Targets.String())
	}
}
