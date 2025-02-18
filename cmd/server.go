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
	"os"
	"os/signal"
	"syscall"
	"time"
	"volcengine-provider/pkg/volcengine"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

// Initialize the start command
var (
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the webhook server",
		Run: func(cmd *cobra.Command, args []string) {
			startServer()
		},
	}

	readTimeOut  int
	writeTimeOut int

	regionID string
)

func init() {
	// Configure Viper
	viper.SetConfigName("config")                   // Name of the config file (without extension)
	viper.SetConfigType("yaml")                     // Config file type
	viper.AddConfigPath(".")                        // Look for config in the current directory
	viper.AddConfigPath("/etc/volcengineprovider/") // Optionally look in /etc

	// Bind flags to the start command
	startCmd.Flags().Int("port", 8080, "Port to listen on")
	startCmd.Flags().Bool("debug", false, "Enable debug logging")
	startCmd.Flags().String("access_key", "", "Access key for remote access")
	startCmd.Flags().String("access_secret", "", "Access secret for remote access")
	startCmd.Flags().IntVarP(&readTimeOut, "read_timeout", "", 60, "Read timeout in seconds")
	startCmd.Flags().IntVarP(&writeTimeOut, "write_timeout", "", 60, "Write timeout in seconds")
	startCmd.Flags().StringVarP(&vpcID, "vpc", "v", "", "related vpc id")
	startCmd.Flags().StringVarP(&regionID, "region", "r", "cn-beijing", "related region id")

	// Bind flags to Viper
	viper.BindPFlag("port", startCmd.Flags().Lookup("port"))
	viper.BindPFlag("debug", startCmd.Flags().Lookup("debug"))
	viper.BindPFlag("access_key", startCmd.Flags().Lookup("access_key"))
	viper.BindPFlag("access_secret", startCmd.Flags().Lookup("access_secret"))
	viper.BindPFlag("vpc", startCmd.Flags().Lookup("vpc"))
	viper.BindPFlag("region", startCmd.Flags().Lookup("region"))
	viper.BindPFlag("endpoint", startCmd.Flags().Lookup("endpoint"))

	// Bind environment variables
	viper.SetEnvPrefix("VOLCENGINE") // Prefix for environment variables
	viper.BindEnv("port")
	viper.BindEnv("debug")
	viper.BindEnv("access_key")
	viper.BindEnv("access_secret")
	viper.BindEnv("vpc")
	viper.BindEnv("region")
	viper.BindEnv("endpoint")
	// Add the start command to the root command
	rootCmd.AddCommand(startCmd)
}

func startServer() {

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		log.Infof("No configuration file found: %v\n", err)
	}
	// Read configuration values
	port := viper.GetInt("port")
	debug := viper.GetBool("debug")
	accessKey := viper.GetString("access_key")
	accessSecret := viper.GetString("access_secret")
	vpcID := viper.GetString("vpc")
	regionID := viper.GetString("region")
	pvzEndpoint := viper.GetString("endpoint")
	if pvzEndpoint == "" {
		pvzEndpoint = "open.volcengineapi.com"
	}

	os.Setenv("VOLC_SECRETKEY", accessSecret) // 为 sdk 设置环境变量
	os.Setenv("VOLC_ACCESSKEY", accessKey)
	// Print debug logs if enabled
	if debug {
		log.SetLevel(log.DebugLevel)
		log.Debugf("Starting server with configuration: port=%d, debug=%t, access_key=%s, access_secret=%s vpc=%s, endpoint=%s, region=%s \n", port, debug, accessKey, accessSecret, vpcID, pvzEndpoint, regionID)
	}

	domainfilter := endpoint.DomainFilter{}
	zoneidfilter := provider.ZoneIDFilter{}

	provider, err := volcengine.NewVolcengineProvider(domainfilter, zoneidfilter, volcengine.VolcengineConfig{
		AccessKeyID:     accessKey,
		AccessKeySecret: accessSecret,
		RegionID:        regionID,
		PrivateZone:     true,
		VPCID:           vpcID,
		Endpoint:        pvzEndpoint,
	})
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGTERM, // 常规终止信号
		syscall.SIGINT,  // Ctrl+C 中断
		// syscall.SIGKILL 不可捕获（内核级信号）
	)
	defer stop()

	startedChan := make(chan struct{})
	go api.StartHTTPApi(
		provider, startedChan,
		time.Duration(readTimeOut)*time.Second,
		time.Duration(writeTimeOut)*time.Second,
		fmt.Sprintf("0.0.0.0:%d", port),
	)

	// Wait for the HTTP server to start and then set the healthy and ready flags
	<-startedChan
	log.Infof("Listening on port %d...\n", port)

	<-ctx.Done()
	log.Infof("Shutting down...\n")
}
