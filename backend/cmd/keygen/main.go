// keygen writes an RSA signing keypair. Supply private and public output paths.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/keygen <private.pem> <public.pem>")
		os.Exit(2)
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.WriteFile(os.Args[1], private, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.WriteFile(os.Args[2], pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
