// Veriqid Bridge — local HTTP API for browser extension communication.
//
// Usage:
//
//	go run ./cmd/bridge -contract 0x... [-port 9090] [-client http://127.0.0.1:7545]
//
// The bridge runs on localhost:9090 and provides JSON endpoints for:
//   - Creating identities (generating msk, registering on-chain)
//   - Generating registration proofs (ring membership + spk)
//   - Generating authentication proofs (Boquila 65-byte proof)
//   - Listing available identity keys
//   - Generating random challenges
//
// SECURITY: The bridge binds to 127.0.0.1 ONLY (not 0.0.0.0).
// It holds secret keys and must not be accessible from the network.
package main

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/patmekury/veriqid/bridge"
)

func main() {
	// CLI flags — same pattern as cmd/server and cmd/client
	contractAddr := flag.String("contract", "", "U2SSO smart contract address (required)")
	clientAddr := flag.String("client", "http://127.0.0.1:7545", "Ethereum JSON-RPC endpoint")
	port := flag.Int("port", 9090, "Port for the bridge API to listen on")
	flag.Parse()

	if *contractAddr == "" {
		log.Fatal("Error: -contract flag is required.\n\nUsage:\n  go run ./cmd/bridge -contract 0xYourContractAddress [-port 9090] [-client http://127.0.0.1:7545]")
	}

	// Create the bridge with defaults
	b := bridge.NewBridge(*contractAddr, *clientAddr)

	// Set up routes
	mux := http.NewServeMux()
	b.RegisterRoutes(mux)

	// Wrap with CORS middleware for browser extension access
	handler := bridge.CORSMiddleware(mux)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	fmt.Println("===========================================")
	fmt.Println("  Veriqid Bridge API")
	fmt.Println("===========================================")
	fmt.Printf("  Listening:  http://%s\n", addr)
	fmt.Printf("  Contract:   %s\n", *contractAddr)
	fmt.Printf("  RPC:        %s\n", *clientAddr)
	fmt.Println("-------------------------------------------")
	fmt.Println("  Endpoints:")
	fmt.Println("    GET  /api/status")
	fmt.Println("    POST /api/identity/create")
	fmt.Println("    POST /api/identity/register")
	fmt.Println("    POST /api/identity/auth")
	fmt.Println("    POST /api/identity/list")
	fmt.Println("    GET  /api/identity/challenge")
	fmt.Println("===========================================")

	// Listen on 127.0.0.1 ONLY — the bridge holds secret keys
	// and must NOT be accessible from other machines on the network.
	log.Fatal(http.ListenAndServe(addr, handler))
}
