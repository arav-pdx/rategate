package main

import (
	"crypto/rsa"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	sub := flag.String("sub", "alice", "Subject client id")
	tier := flag.String("tier", "free", "Tier (free, pro, internal)")
	privKeyPath := flag.String("key", "certs/jwt_private.pem", "Private key path")
	flag.Parse()

	keyBytes, err := os.ReadFile(*privKeyPath)
	if err != nil {
		panic(err)
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		panic(err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  *sub,
		"tier": *tier,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	})

	tokenStr, err := token.SignedString(privKey)
	if err != nil {
		panic(err)
	}
	fmt.Print(tokenStr)
}
