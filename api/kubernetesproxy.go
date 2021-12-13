package api

import (
	"crypto/tls"
	"encoding/pem"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/sirupsen/logrus"
)

type k8handle struct {
	reverseProxy string
}

func (k8 k8handle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.Debug(k8.reverseProxy + " " + r.Method + " " + r.URL.String() + " " + r.Proto + " " + r.UserAgent())
	remote, err := url.Parse(k8.reverseProxy)
	if err != nil {
		logrus.Fatalln(err)
	}

	// Client certificates
	certs := r.TLS.PeerCertificates

	c := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[0].Raw})
	d := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certs[1].Raw})

	logrus.Debug("HTTP CERTS C", string(c))
	logrus.Debug("HTTP CERTS D", string(d))

	cert := tls.Certificate{
		Certificate: [][]byte{c, d},
	}

	logrus.Debug("HTTP CERTS", certs)

	proxy := httputil.NewSingleHostReverseProxy(remote)
	// #nosec G402
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipSSL,
			Certificates:       []tls.Certificate{cert},
		},
	}
	r.Host = remote.Host

	proxy.ServeHTTP(w, r)
}
