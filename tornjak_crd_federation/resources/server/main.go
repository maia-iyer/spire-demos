package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"os"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func index(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received", svidClaims(r.Context()))
	_, _ = io.WriteString(w, "Success!!!")
}

type svidClaimsKey struct{}

func withSVIDClaims(ctx context.Context, claims map[string]interface{}) context.Context {
	return context.WithValue(ctx, svidClaimsKey{}, claims)
}

func svidClaims(ctx context.Context) map[string]interface{} {
	claims, _ := ctx.Value(svidClaimsKey{}).(map[string]interface{})
	return claims
}

func main() {
	var (
		socketPath string
		spiffeId string
	)
	socketPath = os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	spiffeId = os.Getenv("spiffeId")

	if err := run(context.Background(), socketPath, spiffeId); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, socketPath string, audienceString string) error {
	fmt.Println("Called run...")
	// Create options to configure Sources to use expected socket path,
	// as default sources will use value environment variable `SPIFFE_ENDPOINT_SOCKET`
	clientOptions := workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath))

	// Create a X509Source using previously create workloadapi client
	fmt.Printf("creating x509 Source from socket path %s\n", socketPath)
	x509Source, err := workloadapi.NewX509Source(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create X509Source: %w", err)
	}
	defer x509Source.Close()
	fmt.Println("created x509 Source")

	// Create a `tls.Config` with configuration to allow TLS communication with client
	tlsConfig := tlsconfig.TLSServerConfig(x509Source)
	server := &http.Server{
		Addr:              ":8443",
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Second * 10,
	}
	fmt.Println("created tlsConfig")

	http.Handle("/", http.HandlerFunc(index))
	fmt.Println("created httpHandler... now serving")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	} else {
		fmt.Println("served")
	}
	return nil
}
