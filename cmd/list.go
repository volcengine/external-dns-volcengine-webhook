package cmd

import (
	"context"
	"fmt"
	"volcengine-provider/pkg/volcengine"

	"github.com/spf13/cobra"
)

var (
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List zone or records",
		Run: func(cmd *cobra.Command, args []string) {
			listZones()
		},
	}

	vpcId string
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.PersistentFlags().StringVar(&vpcId, "vpc", "", "VPC ID")
}

func listZones() {
	client := volcengine.NewPrivateZoneWrapper()

	// zones, err := client.GetPrivateZones(context.Background())
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }
	// for _, zone := range zones {
	// 	fmt.Printf("Zone ID: %d, Zone Name: %s\n", *zone.ZID, *zone.ZoneName)
	// }

	endpoints, err := volcengine.Records(context.Background(), client, vpcId)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	for _, ep := range endpoints {
		fmt.Printf("dns:%s endpoints:%s\n", ep.DNSName, ep.Targets.String())
	}
}
