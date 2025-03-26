package manager

import (
	"chainlink-ocr-checker/cmd/version"
	"chainlink-ocr-checker/config"
	"chainlink-ocr-checker/utils"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	cli "github.com/spf13/cobra"
)

var (
	configFilePath string
	logLevel       string
	output         string
	cfg            *config.Config
)

var rootCmd = &cli.Command{
	Use:     "ocr-checker",
	Short:   "ocr-checker",
	Long:    `ocr-checker`,
	Version: fmt.Sprintf("Version: %s\nCommit: %s\nDate: %s", version.AppVersion, version.GitCommit, version.BuildDate),
	PersistentPreRun: func(cmd *cli.Command, args []string) {

		err := config.InitializeViper()
		utils.OrShutdown(err)

		cfg, err = config.NewConfig(configFilePath)
		if err != nil {
			log.Fatalln(err)
		}
		err = cfg.LoadConfig()
		utils.OrShutdown(err)

		if logLevel != "" {
			cfg.LogLevel = logLevel
		}
		if cfg.LogLevel == "" {
			cfg.LogLevel = "info"
		}
		if output != "" {
			cfg.OutputFormat = output
		}

		log.SetLevel(utils.LogLevel(cfg.LogLevel))

	},
}

func init() {

	rootCmd.PersistentFlags().StringVarP(&logLevel, config.LOG_LEVEL_FLAG, config.SHORT_LOG_LEVEL_FLAG, "", "Logging level (error, warn, info, debug).")
	rootCmd.PersistentFlags().StringVarP(&output, config.OUTPUT_TYPE_FLAG, config.SHORT_OUTPUT_TYPE_FLAG, "", "Output type (text, json).")
	rootCmd.PersistentFlags().StringVarP(&configFilePath, config.CONFIG_FILE_FLAG, config.SHORT_CONFIG_FILE_FLAG, "config.toml", "Path to the configuration file (default: config.toml).")

	rootCmd.AddCommand(fetchCmd, watchCmd, parseCmd)

}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer cfg.Network.Close()
}
