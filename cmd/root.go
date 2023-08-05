/*
Copyright © 2023 YAUHEN SHULITSKI
*/
package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootPort int
var rootAppID string
var rootAppSecret string

// Logger is the main logger
var Logger *log.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gocycleapp",
	Short: "go-cycle Strava application",
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gocycleapp.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().IntVarP(&rootPort, "port", "p", 8080, "Port to start the server on")
	rootCmd.Flags().StringVarP(&rootAppID, "id", "i", "", "Strava application ID")
	rootCmd.Flags().StringVarP(&rootAppSecret, "secret", "s", "", "Strava application secret")

	Logger = log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile)
}
