package attest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"time"

	"github.com/fxamacker/cbor/v2"
)

//go:embed apple_app_attestation_root.pem
var appleRootPEM []byte

var (
	oidAppleNonce = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 2}
	aaguidProd    = append([]byte("appattest"), make([]byte, 7)...)
	aaguidDev     = []byte("appattestdevelop")
)

type appleAttestation struct {
	Fmt     string `cbor:"fmt"`
	AttStmt struct {
		X5C     [][]byte `cbor:"x5c"`
		Receipt []byte   `cbor:"receipt"`
	} `cbor:"attStmt"`
	AuthData []byte `cbor:"authData"`
}

// VerifyAppAttest checks an App Attest attestation object against Apple's root, the server
// challenge, and this app's identity (teamID.bundleID), following Apple's validation steps.
func VerifyAppAttest(attestation []byte, keyID []byte, challenge []byte, appID string, allowDev bool, now time.Time) error {
	var a appleAttestation
	if err := cbor.Unmarshal(attestation, &a); err != nil {
		return err
	}
	if a.Fmt != "apple-appattest" || len(a.AttStmt.X5C) < 2 {
		return errors.New("not an app attest object")
	}
	leaf, err := x509.ParseCertificate(a.AttStmt.X5C[0])
	if err != nil {
		return err
	}
	inter := x509.NewCertPool()
	for _, c := range a.AttStmt.X5C[1:] {
		ic, err := x509.ParseCertificate(c)
		if err != nil {
			return err
		}
		inter.AddCert(ic)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(appleRootPEM) {
		return errors.New("bad apple root")
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		return err
	}

	clientHash := sha256.Sum256(challenge)
	nonce := sha256.Sum256(append(append([]byte{}, a.AuthData...), clientHash[:]...))
	var got []byte
	for _, ext := range leaf.Extensions {
		if ext.Id.Equal(oidAppleNonce) {
			var seq struct {
				Nonce []byte `asn1:"tag:1,explicit"`
			}
			if _, err := asn1.Unmarshal(ext.Value, &seq); err != nil {
				return err
			}
			got = seq.Nonce
		}
	}
	if !bytes.Equal(got, nonce[:]) {
		return errors.New("nonce mismatch")
	}

	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("unexpected key type")
	}
	point := elliptic.Marshal(pub.Curve, pub.X, pub.Y) //nolint:staticcheck // uncompressed point is what Apple hashes
	keyHash := sha256.Sum256(point)
	if !bytes.Equal(keyHash[:], keyID) {
		return errors.New("key id mismatch")
	}

	ad := a.AuthData
	if len(ad) < 55 {
		return errors.New("short authData")
	}
	rp := sha256.Sum256([]byte(appID))
	if !bytes.Equal(ad[:32], rp[:]) {
		return errors.New("app id mismatch")
	}
	if binary.BigEndian.Uint32(ad[33:37]) != 0 {
		return errors.New("counter not zero")
	}
	aaguid := ad[37:53]
	if !bytes.Equal(aaguid, aaguidProd) && !(allowDev && bytes.Equal(aaguid, aaguidDev)) {
		return errors.New("wrong environment")
	}
	credLen := int(binary.BigEndian.Uint16(ad[53:55]))
	if len(ad) < 55+credLen || !bytes.Equal(ad[55:55+credLen], keyID) {
		return errors.New("credential id mismatch")
	}
	return nil
}
