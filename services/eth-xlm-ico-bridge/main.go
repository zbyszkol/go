package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stellar/go/services/eth-xlm-ico-bridge/backend"
	"github.com/stellar/go/support/log"
)

type Config struct {
	Port     int `valid:"required"`
	Database struct {
		Type string `valid:"matches(^mysql|postgres$)"`
		DSN  string `valid:"required"`
	} `valid:"required"`
}

var rootCmd = &cobra.Command{
	Use:   "eth-xlm-ico-bridge",
	Short: "Bridge server to allow participating in Stellar based ICOs using Ethereum",
}

var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Starts backend server",
	Long: `Backend server listens for Ethereum transactions and funds Stellar account when new ETH transaction appears.
This server should not be accessible from the Internet.`,
	Run: func(cmd *cobra.Command, args []string) {
		server := backend.Server{}
		server.Start()
	},
}

var frontendCmd = &cobra.Command{
	Use:   "frontend",
	Short: "Starts frontend server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("frontend")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("version")
	},
}

func init() {
	log.SetLevel(log.InfoLevel)

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(backendCmd)
	rootCmd.AddCommand(frontendCmd)

	rootCmd.PersistentFlags().StringP("config", "c", "ico.cfg", "config file path")
}

func main() {
	rootCmd.Execute()
}
