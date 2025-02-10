package cmd

import (
	"fmt"
	"net/http"
	"time"
	"volcengine-provider/pkg/volcengine"

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

	// Bind flags to Viper
	viper.BindPFlag("port", startCmd.Flags().Lookup("port"))
	viper.BindPFlag("debug", startCmd.Flags().Lookup("debug"))
	viper.BindPFlag("access_key", startCmd.Flags().Lookup("access_key"))
	viper.BindPFlag("access_secret", startCmd.Flags().Lookup("access_secret"))

	// Bind environment variables
	viper.SetEnvPrefix("VOLCENGINE") // Prefix for environment variables
	viper.BindEnv("port")
	viper.BindEnv("debug")
	viper.BindEnv("access_key")
	viper.BindEnv("access_secret")

	// Add the start command to the root command
	rootCmd.AddCommand(startCmd)
}

func startServer() {

	// Read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("No configuration file found: %v\n", err)
	}
	// Read configuration values
	port := viper.GetInt("port")
	debug := viper.GetBool("debug")
	accessKey := viper.GetString("access_key")
	accessSecret := viper.GetString("access_secret")

	// Print debug logs if enabled
	if debug {
		fmt.Printf("Starting server with configuration: port=%d, debug=%t, access_key=%s, access_secret=%s\n", port, debug, accessKey, accessSecret)
	}

	domainfilter := endpoint.DomainFilter{}
	zoneidfilter := provider.ZoneIDFilter{}

	provider, err := volcengine.NewVolcengineProvider(domainfilter, zoneidfilter, volcengine.VolcengineConfig{})
	if err != nil {
		panic(err)
	}

	startedChan := make(chan struct{})
	go api.StartHTTPApi(
		provider, startedChan,
		time.Duration(readTimeOut)*time.Second,
		time.Duration(writeTimeOut)*time.Second,
		fmt.Sprintf("0.0.0.0:%d", port),
	)

	fmt.Printf("Listening on port %d...\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		panic(err)
	}
}
