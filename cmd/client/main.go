package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"

	u2sso "github.com/patmekury/veriqid/pkg/u2sso"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// #cgo CFLAGS: -g -Wall
// #cgo LDFLAGS: -lcrypto -lsecp256k1
// #include <stdlib.h>
// #include <stdint.h>
// #include <string.h>
// #include <openssl/rand.h>
// #include <secp256k1.h>
// #include <secp256k1_ringcip.h>
import "C"

var contractFlag string
var clientFlag string
var skFlag string
var commandFlag string
var passkeypathFlag string
var snameFlag string
var challengeFlag string

func main() {
	commands := []string{"create", "load", "register", "auth", "help"}

	flag.StringVar(&contractFlag, "contract", "", "The hexAddress of your contract, e.g., 0xf...")
	flag.StringVar(&clientFlag, "client", "", "The ethereum client IP of your contract")
	flag.StringVar(&skFlag, "ethkey", "", "The private key of your account (without 0x), e.g., f...")
	flag.StringVar(&commandFlag, "command", "", "Choose commands: create, load, register, auth, help")
	flag.StringVar(&passkeypathFlag, "keypath", "", "Path to store/load your SSO-Id passkey")
	flag.StringVar(&snameFlag, "sname", "", "Service name (hex-encoded SHA-256) for register or authenticate")
	flag.StringVar(&challengeFlag, "challenge", "", "Challenge (hex-encoded) given by the service")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var client *ethclient.Client
	var err error
	var instance *u2sso.U2sso
	var idsize *big.Int

	if commandFlag == "" {
		fmt.Println("Please provide a command using the -command flag from", commands)
		return
	}

	if clientFlag == "" {
		fmt.Println("No client address was given. Taking default: http://127.0.0.1:7545")
		clientFlag = "http://127.0.0.1:7545"
		client, err = ethclient.Dial("http://127.0.0.1:7545")
		if err != nil {
			fmt.Println("Error connecting to eth client:", err)
		}
	}

	if contractFlag == "" {
		fmt.Println("Please provide a contract address using the -contract flag")
		return
	} else {
		contractAddress := common.HexToAddress(contractFlag)
		bytecode, err := client.CodeAt(context.Background(), contractAddress, nil)
		if err != nil {
			log.Fatal(err)
		}
		isContract := len(bytecode) > 0
		instance, err = u2sso.NewU2sso(contractAddress, client)
		if err != nil || !isContract {
			fmt.Println("No U2SSO contract at", contractFlag)
			return
		}
		fmt.Println("Found the contract at", contractFlag)

		idsize, err = instance.GetIDSize(nil)
		if err != nil {
			fmt.Println("Could not get id size from", contractFlag)
		}
		fmt.Println("Current id size:", idsize)
	}

	if commandFlag == "create" {
		if passkeypathFlag == "" {
			fmt.Println("Please provide a passkeypath using the -keypath flag to store your passkey")
			return
		}
		if skFlag == "" {
			fmt.Println("Please provide your ethereum account private key using the -ethkey flag (without 0x)")
			return
		}

		u2sso.CreatePasskey(passkeypathFlag)
		mskBytes, val := u2sso.LoadPasskey(passkeypathFlag)
		if !val {
			fmt.Println("Could not create and load passkey")
			return
		}
		fmt.Println("Passkey created successfully")

		idBytes := u2sso.CreateID(mskBytes)
		fmt.Println("Added ID to index:", u2sso.AddIDstoIdR(client, skFlag, instance, idBytes))
		return
	}

	if commandFlag == "load" {
		if passkeypathFlag == "" {
			fmt.Println("Please provide a passkeypath using the -keypath flag to store your passkey")
			return
		}
		mskBytes, val := u2sso.LoadPasskey(passkeypathFlag)
		if !val {
			fmt.Println("Could not load passkey")
			return
		}
		fmt.Println("Passkey loaded successfully, length:", len(mskBytes))
	}

	if commandFlag == "register" {
		if passkeypathFlag == "" {
			fmt.Println("Please provide a passkeypath using the -keypath flag")
			return
		}
		if snameFlag == "" {
			fmt.Println("Please provide a service name using the -sname flag (hex-encoded SHA-256)")
			return
		}
		serviceName, err := hex.DecodeString(snameFlag)
		if err != nil {
			fmt.Println("Please provide a valid service name of hex characters")
			return
		}

		if challengeFlag == "" {
			fmt.Println("Please provide a challenge using the -challenge flag (hex-encoded)")
			return
		}
		challenge, err := hex.DecodeString(challengeFlag)
		if err != nil {
			fmt.Println("Please provide a valid challenge of hex characters")
			return
		}

		idsize, err = instance.GetIDSize(nil)
		if err != nil {
			fmt.Println("Could not get id size from contract")
			return
		}
		fmt.Println("Total ID size:", idsize.Int64())
		if idsize.Int64() < 2 {
			fmt.Println("At least two SSO-Ids are required. Create more identities first.")
			return
		}

		currentm := 1
		ringSize := 1
		for i := 1; i < u2sso.M; i++ {
			ringSize = u2sso.N * ringSize
			if ringSize >= int(idsize.Int64()) {
				currentm = i
				break
			}
		}
		fmt.Println("Chosen ring size:", idsize.Int64(), "and m:", currentm)

		mskBytes, val := u2sso.LoadPasskey(passkeypathFlag)
		if !val {
			fmt.Println("Could not load passkey")
			return
		}

		idBytes := u2sso.CreateID(mskBytes)
		index := u2sso.GetIDIndexfromContract(instance, idBytes)
		if index == -1 {
			fmt.Println("The SSO-id for this passkey is not registered in the contract")
			return
		}

		IdList := u2sso.GetallActiveIDfromContract(instance)
		proofHex, spkBytes, val := u2sso.RegistrationProof(int(index), currentm, int(idsize.Int64()), serviceName, challenge, mskBytes, IdList)
		fmt.Println("Proof hex:", proofHex)
		fmt.Println("SPK hex:", hex.EncodeToString(spkBytes))
		fmt.Println("N:", idsize.Int64())
	}

	if commandFlag == "auth" {
		if passkeypathFlag == "" {
			fmt.Println("Please provide a passkeypath using the -keypath flag")
			return
		}
		if snameFlag == "" {
			fmt.Println("Please provide a service name using the -sname flag (hex-encoded SHA-256)")
			return
		}
		serviceName, err := hex.DecodeString(snameFlag)
		if err != nil {
			fmt.Println("Please provide a valid service name of hex characters")
			return
		}

		if challengeFlag == "" {
			fmt.Println("Please provide a challenge using the -challenge flag (hex-encoded)")
			return
		}
		challenge, err := hex.DecodeString(challengeFlag)
		if err != nil {
			fmt.Println("Please provide a valid challenge of hex characters")
			return
		}

		mskBytes, val := u2sso.LoadPasskey(passkeypathFlag)
		if !val {
			fmt.Println("Could not load passkey")
			return
		}

		proofAuthHex, val := u2sso.AuthProof(serviceName, challenge, mskBytes)
		fmt.Println("Auth proof hex:", proofAuthHex)
	}

	if commandFlag == "help" {
		fmt.Println("Veriqid Client - Commands:")
		fmt.Println("  create   - Create a new identity (requires -keypath, -ethkey)")
		fmt.Println("  load     - Load an existing identity (requires -keypath)")
		fmt.Println("  register - Register with a service (requires -keypath, -sname, -challenge)")
		fmt.Println("  auth     - Authenticate with a service (requires -keypath, -sname, -challenge)")
		fmt.Println("  help     - Show this help message")
	}
}
