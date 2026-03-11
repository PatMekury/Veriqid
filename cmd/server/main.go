package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

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

type account struct {
	name       string
	spk        []byte
	nullifier  []byte
	currentn   int
	proof      string
	userip     string
	challenge  []byte
	registered bool
}

type authchallenges struct {
	authChallenge []byte
	userip        string
}

var registeredSPK = make([]account, 0)
var regAuthChallenges = make([]authchallenges, 0)

func newAccount(challenge []byte, userip string) account {
	acc := account{challenge: challenge, userip: userip}
	return acc
}

func newAuthChallenges(chal []byte, userip string) authchallenges {
	au := authchallenges{authChallenge: []byte(chal), userip: userip}
	return au
}

const sname = "abc_service"

var contractSFlag string
var clientSFlag string

var instanceS *u2sso.Veriqid

func main() {
	flag.StringVar(&contractSFlag, "contract", "", "The hexAddress of your contract, e.g., 0xf...")
	flag.StringVar(&clientSFlag, "client", "", "The ethereum client IP of your contract")

	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var client *ethclient.Client
	var err error
	var idsize *big.Int

	if clientSFlag == "" {
		fmt.Println("No client address was given. Taking default: http://127.0.0.1:7545")
		clientSFlag = "http://127.0.0.1:7545"
		client, err = ethclient.Dial("http://127.0.0.1:7545")
		if err != nil {
			fmt.Println("Error connecting to eth client:", err)
		}
	}

	if contractSFlag == "" {
		fmt.Println("Please provide a contract address using the -contract flag")
		return
	} else {
		contractAddress := common.HexToAddress(contractSFlag)
		bytecode, err := client.CodeAt(context.Background(), contractAddress, nil)
		if err != nil {
			log.Fatal(err)
		}
		isContract := len(bytecode) > 0
		instanceS, err = u2sso.NewVeriqid(contractAddress, client)
		if err != nil || !isContract {
			fmt.Println("No Veriqid contract at", contractSFlag)
			return
		}
		fmt.Println("Found the contract at", contractSFlag)

		idsize, err = instanceS.GetIDSize(nil)
		if err != nil {
			fmt.Println("Could not get id size from", contractSFlag)
		}
		fmt.Println("Current id size:", idsize)
	}

	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", fileServer)
	http.HandleFunc("/directLogin", loginFormHandler)
	http.HandleFunc("/directSignup", signupFormHandler)
	http.HandleFunc("/signup", signupHandler)
	http.HandleFunc("/login", loginHandler)

	fmt.Printf("Veriqid server started at http://localhost:8080\n")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func signupFormHandler(w http.ResponseWriter, r *http.Request) {
	challenge := u2sso.CreateChallenge()
	acc := newAccount(challenge, userIP(r))
	registeredSPK = append(registeredSPK, acc)

	sha := sha256.New()
	sha.Write([]byte(sname))
	serviceName := hex.EncodeToString(sha.Sum(nil))

	htmlFilePath := "./static/signup.html"
	htmlContent, err := os.ReadFile(htmlFilePath)
	if err != nil {
		http.Error(w, "Could not load signup page", http.StatusInternalServerError)
		return
	}

	htmlContentStr := string(htmlContent)
	htmlContentStr = strings.Replace(htmlContentStr, "{{CHALLENGE}}", hex.EncodeToString(challenge), -1)
	htmlContentStr = strings.Replace(htmlContentStr, "{{SERVICE_NAME}}", serviceName, -1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlContentStr)
}

func loginFormHandler(w http.ResponseWriter, r *http.Request) {
	challenge := u2sso.CreateChallenge()
	au := newAuthChallenges(challenge, userIP(r))
	regAuthChallenges = append(regAuthChallenges, au)

	sha := sha256.New()
	sha.Write([]byte(sname))
	serviceName := hex.EncodeToString(sha.Sum(nil))

	htmlFilePath := "./static/login.html"
	htmlContent, err := os.ReadFile(htmlFilePath)
	if err != nil {
		http.Error(w, "Could not load login page", http.StatusInternalServerError)
		return
	}

	htmlContentStr := string(htmlContent)
	htmlContentStr = strings.Replace(htmlContentStr, "{{CHALLENGE}}", hex.EncodeToString(challenge), -1)
	htmlContentStr = strings.Replace(htmlContentStr, "{{SERVICE_NAME}}", serviceName, -1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlContentStr)
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	name := r.FormValue("name")
	challenge, err := hex.DecodeString(r.FormValue("challenge"))
	if err != nil {
		fmt.Fprintf(w, "Challenge error: %v\nSign up request failed\n", err)
		return
	}
	spkBytes, err := hex.DecodeString(r.FormValue("spk"))
	if err != nil {
		fmt.Fprintf(w, "SPK error: %v\nSign up request failed\n", err)
		return
	}
	serviceName, err := hex.DecodeString(r.FormValue("sname"))
	if err != nil {
		fmt.Fprintf(w, "Service name error: %v\nSign up request failed\n", err)
		return
	}
	nullifier := []byte(r.FormValue("nullifier")) // todo: no nullifiers in the basic version
	proofHex := r.FormValue("proof")
	ringSize, err := strconv.Atoi(r.FormValue("n"))
	if err != nil {
		fmt.Fprintf(w, "Ring size error: %v\nSign up request failed\n", err)
		return
	}

	currentm := 1
	tmp := 1
	for currentm = 1; currentm < u2sso.M; currentm++ {
		tmp *= u2sso.N
		if tmp >= ringSize {
			break
		}
	}

	IdList, err := u2sso.GetallActiveIDfromContract(instanceS)
	if err != nil {
		fmt.Fprintf(w, "Could not get IDs from contract: %v\nSign up request failed\n", err)
		return
	}
	res := u2sso.RegistrationVerify(proofHex, currentm, ringSize, serviceName, challenge, IdList, spkBytes)

	if res {
		for i := 0; i < len(registeredSPK); i++ {
			if bytes.Compare(registeredSPK[i].challenge, challenge) == 0 {
				registeredSPK[i].name = name
				registeredSPK[i].spk = spkBytes
				registeredSPK[i].nullifier = nullifier
				registeredSPK[i].proof = proofHex
				registeredSPK[i].registered = true
			}
		}
		htmlFilePath := "./static/registration_success.html"
		htmlContent, err := os.ReadFile(htmlFilePath)
		if err != nil {
			http.Error(w, "Could not load registration success page", http.StatusInternalServerError)
			return
		}
		htmlContentStr := strings.Replace(string(htmlContent), "{{NAME}}", name, -1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContentStr)
	} else {
		htmlFilePath := "./static/registration_fail.html"
		htmlContent, err := os.ReadFile(htmlFilePath)
		if err != nil {
			http.Error(w, "Could not load registration fail page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, string(htmlContent))
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}

	name := r.FormValue("name")
	challenge, err := hex.DecodeString(r.FormValue("challenge"))
	if err != nil {
		fmt.Fprintf(w, "Challenge error: %v\nLogin request failed\n", err)
		return
	}
	spkBytes, err := hex.DecodeString(r.FormValue("spk"))
	if err != nil {
		fmt.Fprintf(w, "SPK error: %v\nLogin request failed\n", err)
		return
	}
	serviceName, err := hex.DecodeString(r.FormValue("sname"))
	if err != nil {
		fmt.Fprintf(w, "Service name error: %v\nLogin request failed\n", err)
		return
	}
	signature := r.FormValue("signature")
	res := u2sso.AuthVerify(signature, serviceName, challenge, spkBytes)
	res2 := false

	for i := 0; i < len(registeredSPK); i++ {
		if bytes.Compare(registeredSPK[i].spk, spkBytes) == 0 {
			res2 = true
		}
	}

	if res && res2 {
		htmlFilePath := "./static/login_success.html"
		htmlContent, err := os.ReadFile(htmlFilePath)
		if err != nil {
			http.Error(w, "Could not load login success page", http.StatusInternalServerError)
			return
		}
		htmlContentStr := strings.Replace(string(htmlContent), "{{NAME}}", name, -1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContentStr)
	} else {
		htmlFilePath := "./static/login_fail.html"
		htmlContent, err := os.ReadFile(htmlFilePath)
		if err != nil {
			http.Error(w, "Could not load login fail page", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, string(htmlContent))
	}
}

func userIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
