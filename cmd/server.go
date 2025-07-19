package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/nomasters/haystack/server"
	"github.com/nomasters/haystack/storage/memory"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringP("port", "p", "1337", "Port for the server listener")
	serverCmd.Flags().StringP("host", "", "", "hostname of server listener")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run haystack in server mode.",
	Long:  `Server mode is used to run long-lived haystack servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetString("port")
		host, _ := cmd.Flags().GetString("host")
		addr := host + ":" + port
		
		fmt.Println("listening on:", addr)
		
		// Create storage backend (memory storage with 24h TTL, 2M max items)
		ctx := context.Background()
		storage := memory.New(ctx, 24*time.Hour, 2000000)
		
		// Create UDP server
		srv := server.New(&server.Config{
			Storage: storage,
		})
		
		// Handle graceful shutdown
		go func() {
			if err := srv.ListenAndServe(addr); err != nil {
				fmt.Printf("Server error: %v\n", err)
				os.Exit(1)
			}
		}()
		
		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		
		fmt.Println("\nShutting down server...")
		
		// Shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := srv.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Error during shutdown: %v\n", err)
		}
		
		// Close storage
		if err := storage.Close(); err != nil {
			fmt.Printf("Error closing storage: %v\n", err)
		}
		
		fmt.Println("Server stopped")
	},
}
