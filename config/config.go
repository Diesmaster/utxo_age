package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	RPCUser     string
	RPCPassword string
	RPCURL      string
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found. Falling back to system env vars.")
	}

	RPCUser = os.Getenv("RPC_USER")
	RPCPassword = os.Getenv("RPC_PASSWORD")
	RPCURL = os.Getenv("RPC_URL")

	if RPCUser == "" || RPCPassword == "" || RPCURL == "" {
		log.Fatal("Missing required RPC environment variables (RPC_USER, RPC_PASSWORD, RPC_URL)")
	}
}

func PrintConfig() {
	fmt.Println("🔧 Loaded RPC Config:")
	fmt.Println("  RPC_USER    =", RPCUser)
	fmt.Println("  RPC_PASSWORD=", RPCPassword)
	fmt.Println("  RPC_URL     =", RPCURL)
}

func maskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}

