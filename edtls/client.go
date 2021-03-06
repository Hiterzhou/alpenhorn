package edtls

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"net"

	"vuvuzela.io/alpenhorn/errors"
)

var (
	ErrNoPeerCertificates = errors.New("peer did not supply a certificate")
	ErrVerificationFailed = errors.New("failed to verify certificate")
)

func Dial(network, addr string, theirKey ed25519.PublicKey, myKey ed25519.PrivateKey) (*tls.Conn, error) {
	config := NewTLSClientConfig(myKey, theirKey)

	return tls.Dial(network, addr, config)
}

func Client(rawConn net.Conn, theirKey ed25519.PublicKey, myKey ed25519.PrivateKey) *tls.Conn {
	config := NewTLSClientConfig(myKey, theirKey)

	conn := tls.Client(rawConn, config)
	return conn
}

func NewTLSClientConfig(myKey ed25519.PrivateKey, peerKey ed25519.PublicKey) *tls.Config {
	var config = &tls.Config{
		RootCAs:            x509.NewCertPool(),
		ClientAuth:         tls.RequestClientCert,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,

		GetClientCertificate: func(req *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			if myKey == nil {
				return &tls.Certificate{}, nil
			}

			certDER, err := newSelfSignedCert(myKey)
			if err != nil {
				return nil, errors.New("error generating self-signed certificate: %s", err)
			}
			cert := &tls.Certificate{
				Certificate: [][]byte{certDER},
				PrivateKey:  myKey,
			}
			return cert, nil
		},

		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return ErrNoPeerCertificates
			}

			if len(rawCerts) != 1 {
				return errors.New("too many peer certificates: %d", len(rawCerts))
			}

			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return errors.Wrap(err, "x509.ParseCertificate")
			}

			if err := cert.CheckSignatureFrom(cert); err != nil {
				return ErrVerificationFailed
			}
			theirKey, ok := cert.PublicKey.(ed25519.PublicKey)
			if !ok {
				return errors.New("invalid public key type in certificate: %T", cert.PublicKey)
			}
			if !bytes.Equal(theirKey, peerKey) {
				return ErrVerificationFailed
			}

			return nil
		},
	}

	return config
}
