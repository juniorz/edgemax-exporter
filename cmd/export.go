package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/juniorz/edgemax-exporter/api"
	"github.com/juniorz/edgemax-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	configHost     string
	configUser     string
	configPassword string

	configTLSSkipVerify bool
	configTLSCACertPath string
)

func init() {
	flag.StringVar(&configHost, "host", "https://192.168.0.1", "EdgeMAX host")
	flag.StringVar(&configUser, "user", "", "Username")
	flag.StringVar(&configPassword, "password", "", "Password")

	flag.BoolVar(&configTLSSkipVerify, "tls-skip-verify", false, "Disable verification of TLS certificates.\nUsing this option is highly discouraged as it decreases the security.")
	flag.StringVar(&configTLSCACertPath, "ca-cert", "", "Path on the local disk to a single PEM-encoded CA certificate to verify the server's SSL certificate.")
}

func buildRootCAs(caPath string) (*x509.CertPool, error) {
	if caPath == "" {
		return nil, nil
	}

	r, err := ioutil.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(r)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool, nil
}

func buildTLSConfig(skipVerify bool, caPath string) (*tls.Config, error) {
	var err error
	var rootCAs *x509.CertPool = nil

	if !skipVerify {
		rootCAs, err = buildRootCAs(caPath)
	}

	return &tls.Config{
		InsecureSkipVerify: skipVerify,
		RootCAs:            rootCAs,
	}, err
}

func readConfigFromEnv() {
	if host, ok := os.LookupEnv("EDGEMAX_HOST"); ok {
		configHost = host
	}

	if user, ok := os.LookupEnv("EDGEMAX_USER"); ok {
		configUser = user
	}

	if pass, ok := os.LookupEnv("EDGEMAX_PASSWORD"); ok {
		configPassword = pass
	}

	if skipVerify, ok := os.LookupEnv("EDGEMAX_SKIP_VERIFY"); ok {
		configTLSSkipVerify = skipVerify == "true"
	}

	if certPath, ok := os.LookupEnv("EDGEMAX_CACERT"); ok {
		configTLSCACertPath = certPath
	}
}

func buildHTTPServer(handler http.Handler) (*http.Server, <-chan struct{}) {
	srv := &http.Server{
		Addr:    ":9745",
		Handler: handler,
	}

	serverTerminated := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGTERM, os.Kill)
		<-sigint

		log.Println("Shutting down...")

		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}

		close(serverTerminated)
	}()

	return srv, serverTerminated
}

func buildRegistryFor(c prometheus.Collector) prometheus.Gatherer {
	// Since we are dealing with custom Collector implementations, it might
	// be a good idea to try it out with a pedantic registry.
	reg := prometheus.NewPedanticRegistry()

	// Wrap with edgemax host
	prometheus.WrapRegistererWith(
		prometheus.Labels{"edgemax_host": configHost}, reg,
	).MustRegister(c)

	// Add the standard process and Go metrics to the custom registry.
	reg.MustRegister(
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
	)

	return reg
}

func buildHandler(c *api.Client) http.Handler {
	metricsHandler := promhttp.HandlerFor(buildRegistryFor(collector.New(c)), promhttp.HandlerOpts{})

	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/healthz", func(resp http.ResponseWriter, req *http.Request) {
		//TODO
	})

	return mux
}

func Execute() {
	readConfigFromEnv()

	tlsConfig, err := buildTLSConfig(configTLSSkipVerify, configTLSCACertPath)
	if err != nil {
		log.Fatalf("error: %s", err)
	}

	// TODO:
	// 1. Debug mode

	mux := buildHandler(&api.Client{
		Host:     configHost,
		Username: configUser,
		Password: configPassword,

		TLSConfig: tlsConfig,
	})

	srv, done := buildHTTPServer(mux)

	log.Printf("Listening on %s\n", srv.Addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}

	<-done
	log.Println("Bye!")
}
