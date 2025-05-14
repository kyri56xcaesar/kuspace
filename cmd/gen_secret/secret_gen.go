package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"strings"
	"time"
)

var (
	cert = flag.Bool("cert", false, "create a RSA key pair?")

	ltoken = flag.Int("l", 32, "how long the token to be?(minimum 4)")

	org   = flag.String("org", "ku_org", "what is your organization")
	cname = flag.String("cn", "localhost", "what is your common name?")
	size  = flag.Int("cert-size", 4096, "how big is your rsa cert?")
	ko    = flag.String("ko", "server.key", "name of key")
	co    = flag.String("co", "server.crt", "name of cert")
	dir   = flag.String("dir", "data/cert/", "path of the generated files")
)

func generate_rsa_keypair() {
	// Generate a new RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, *size)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   *cname,
			Organization: []string{*org},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create a self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	// Save the certificate to a .crt file
	certOut, err := os.Create(*dir + *co)
	if err != nil {
		log.Fatalf("Failed to open cert file for writing: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()
	log.Printf("Saved certificate to %s\n", *dir+*co)

	// Save the private key to a .key file
	keyOut, err := os.Create(*dir + *ko)
	if err != nil {
		log.Fatalf("Failed to open key file for writing: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	keyOut.Close()
	log.Printf("Saved private key to %s\n", *dir+*ko)
}

func generate_random_token() {

	tokenbytes := make([]byte, *ltoken)
	if _, err := io.ReadFull(rand.Reader, tokenbytes); err != nil {
		panic(err)
	}

	token := hex.EncodeToString(tokenbytes)
	fmt.Println(token)
}

func main() {
	parseFlags()
	if *cert {
		generate_rsa_keypair()
	} else {
		generate_random_token()
	}
}

func usage() {
	fmt.Println("Usage of random data generator:")
	flag.PrintDefaults()
}
func parseFlags() {
	flag.Usage = usage
	flag.Parse()

	*size = max(min(*size, 4096), 1024)
	*ltoken = max(min(*ltoken, 512), 4)

	if !strings.HasSuffix(*dir, "/") {
		*dir = *dir + "/"
	}

}
