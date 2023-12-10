// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

// tailscale-reverse-proxy is a tailscale node creator that reverse proxies
// HTTP services.
//
// Set the TS_AUTHKEY environment variable to have this server automatically
// join your tailnet, or look for the logged auth link on first start.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/jszwec/s3fs"
	"tailscale.com/tsnet"
)

var (
	hostname     = flag.String("hostname", "", "Tailscale hostname to serve on, used as the base name for MagicDNS or subdomain in your domain alias for HTTPS.")
	bucket       = flag.String("bucket", "", "The S3 bucket to serve from. See -key-prefix if you need to serve part of it.")
	keyPrefix    = flag.String("key-prefix", "", "Prefix for the keys in the bucket to serve")
	tailscaleDir = flag.String("state-dir", "./", "Alternate directory to use for Tailscale state storage. If empty, a default is used.")
	useHTTPS     = flag.Bool("use-https", false, "Serve over HTTPS via your *.ts.net subdomain if enabled in Tailscale admin.")
	debug        = flag.Bool("debug", false, "Print out HTTP requests as they come in")
)

func main() {
	// used a lot
	var err error

	flag.Parse()
	if *hostname == "" || strings.Contains(*hostname, ".") {
		log.Fatal("missing or invalid -hostname")
	}
	if *bucket == "" {
		log.Fatal("missing -bucket")
	}

	region, exists := os.LookupEnv("AWS_REGION")
	if !exists {
		region = "us-west-2"
	}

	s := session.Must(session.NewSession(&aws.Config{Region: &region}))

	ident, err := sts.New(s).GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Using AWS identity: %s", *ident.Arn)

	svc := s3.New(s)
	s3fs := s3fs.New(svc, *bucket)

	ts := &tsnet.Server{
		Dir:      *tailscaleDir,
		Hostname: *hostname,
	}

	if err := ts.Start(); err != nil {
		log.Fatalf("Error starting tsnet.Server: %v", err)
	}
	localClient, _ := ts.LocalClient()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if *keyPrefix != "" {
			r.URL.Path = singleJoiningSlash(*keyPrefix, r.URL.Path)
		}

		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/")

		http.FileServer(http.FS(s3fs)).ServeHTTP(w, r)
	})

	var ln net.Listener

	if *useHTTPS {
		ln, err = ts.Listen("tcp", ":443")
		if err != nil {
			log.Fatal(err)
		}

		ln = tls.NewListener(ln, &tls.Config{
			GetCertificate: localClient.GetCertificate,
		})

		go func() {
			// wait for tailscale to start before trying to fetch cert names
			for i := 0; i < 60; i++ {
				st, err := localClient.Status(context.Background())
				if err != nil {
					log.Printf("error retrieving tailscale status; retrying: %v", err)
				} else {
					log.Printf("tailscale status: %v", st.BackendState)
					if st.BackendState == "Running" {
						break
					}
				}
				time.Sleep(time.Second)
			}

			l80, err := ts.Listen("tcp", ":80")
			if err != nil {
				log.Fatal(err)
			}
			name, ok := localClient.ExpandSNIName(context.Background(), *hostname)
			if !ok {
				log.Fatalf("can't get hostname for https redirect")
			}
			if err := http.Serve(l80, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, fmt.Sprintf("https://%s", name), http.StatusMovedPermanently)
			})); err != nil {
				log.Fatal(err)
			}
		}()
	} else {
		ln, err = ts.Listen("tcp", ":80")
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tailscale-s3-proxy running at %v, proxying to %v", ln.Addr(), *bucket)
	log.Fatal(http.Serve(ln, handler))
}

func printRequest(req *http.Request) {
	log.Printf("%s %s %s", req.Method, req.RemoteAddr, req.RequestURI)
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
