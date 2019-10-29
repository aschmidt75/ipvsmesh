package cmd

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	cli "github.com/jawher/mow.cli"
	log "github.com/sirupsen/logrus"
)

// GenKeyFiles generates a EC P256 private key and x509 certificate, writes to files
// with given prefix. CN is set, lifetime is 1yr
func GenKeyFiles(prefix, cn string) error {
	var priv *ecdsa.PrivateKey
	var err error

	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pubk := &priv.PublicKey

	if err != nil {
		return err
	}

	var notBefore = time.Now()
	var notAfter = notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, pubk, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(fmt.Sprintf("%s.cert", prefix))
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}

	keyOut, err := os.OpenFile(fmt.Sprintf("%s.key", prefix), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}); err != nil {
		return err
	}
	if err := keyOut.Close(); err != nil {
		return err
	}

	return nil
}

// CmdGenerateKey implements the generation for a tls key
func CmdGenerateKey(cmd *cli.Cmd) {
	cmd.Action = func() {
		if err := GenKeyFiles("ipvsmesh", "ipvsmesh-grpc-tls-comm"); err != nil {
			log.WithField("err", err).Error("unable to generate/write key/certificate file")
		}
	}
}
