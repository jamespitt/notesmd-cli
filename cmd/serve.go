package cmd

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
	"github.com/Yakitrak/notesmd-cli/pkg/server"
	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start an HTTP API server for the vault",
	Run: func(cmd *cobra.Command, args []string) {
		vault := &obsidian.Vault{Name: vaultName}
		note := &obsidian.Note{}

		srv := server.New(vault, note)
		addr := fmt.Sprintf(":%d", servePort)

		log.Printf("notesmd-cli server listening on %s", addr)
		if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 7070, "port to listen on")
	serveCmd.Flags().StringVarP(&vaultName, "vault", "v", "", "vault name (uses default if not set)")
	rootCmd.AddCommand(serveCmd)
}
