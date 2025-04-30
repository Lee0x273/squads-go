package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	multisigcreate "github.com/hogyzen12/squads-go/cmd/multisig-create"
	multisiginfo "github.com/hogyzen12/squads-go/cmd/multisig-info"
	multisigtransaction "github.com/hogyzen12/squads-go/cmd/multisig-transaction"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	rootCmd := &cobra.Command{
		Use:   "squads-cli",
		Short: "CLI for Squads Multisig Protocol",
	}

	// Global persistent flags that can be used across all commands
	rootCmd.PersistentFlags().String("rpc", "https://api.mainnet-beta.solana.com", "Solana RPC endpoint")
	rootCmd.PersistentFlags().String("ws", "wss://api.mainnet-beta.solana.com", "Solana WebSocket endpoint")

	// Create a multisig command group
	multisigCmd := &cobra.Command{
		Use:   "multisig",
		Short: "Manage Squads Multisig wallets",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add multisig subcommands
	multisigCmd.AddCommand(
		multisigcreate.NewCommand(),
		multisiginfo.NewCommand(),
	)

	// Create a transaction subcommand group
	transactionCmd := &cobra.Command{
		Use:   "transaction",
		Short: "Manage Squads Multisig transactions",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// Add transaction subcommands
	transactionCmd.AddCommand(
		multisigtransaction.NewCreateCommand(),
		multisigtransaction.NewApproveCommand(),
		multisigtransaction.NewExecuteCommand(),
	)

	// Add command groups to root
	rootCmd.AddCommand(
		multisigCmd,
		transactionCmd,
	)

	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
