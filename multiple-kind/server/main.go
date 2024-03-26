package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"os"

	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func index(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received", svidClaims(r.Context()))
	_, _ = io.WriteString(w, "Success!!!")
}

type authenticator struct {
	// JWT Source used to verify token
	jwtSource *workloadapi.JWTSource
	// Expected audiences
	audiences []string
}

func (a *authenticator) authenticateClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fields := strings.Fields(req.Header.Get("Authorization"))
		if len(fields) != 2 || fields[0] != "Bearer" {
			log.Print("Malformed header")
			http.Error(w, "Invalid or unsupported authorization header", http.StatusUnauthorized)
			return
		}

		token := fields[1]

		// Parse and validate token against fetched bundle from jwtSource,
		// an alternative is using `workloadapi.ValidateJWTSVID` that will
		// attest against SPIRE on each call and validate token
		svid, err := jwtsvid.ParseAndValidate(token, a.jwtSource, a.audiences)
		if err != nil {
			log.Printf("Invalid token: %v\n", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		req = req.WithContext(withSVIDClaims(req.Context(), svid.Claims))
		next.ServeHTTP(w, req)
	})
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

	// Create a JWTSource to validate provided tokens from clients
	jwtSource, err := workloadapi.NewJWTSource(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create JWTSource: %w", err)
	}
	defer jwtSource.Close()
	fmt.Println("created jwt Source")

	// Add a middleware to validate presented JWT token
	auth := &authenticator{
		jwtSource: jwtSource,
		audiences: []string{audienceString},
	}
	fmt.Println("created middleware")
	http.Handle("/", auth.authenticateClient(http.HandlerFunc(index)))
	fmt.Println("created httpHandler... now serving")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	} else {
		fmt.Println("served")
	}
	return nil
}
