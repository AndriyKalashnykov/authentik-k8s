package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	httptransport "github.com/go-openapi/runtime/client"
	api "goauthentik.io/api/v3"
)

// APIClient Hold the API Client and any relevant configuration
type APIClient struct {
	client *api.APIClient
}
type tracingTransport struct {
	inner http.RoundTripper
	ctx   context.Context
}

// GetTLSTransport Get a TLS transport instance, that skips verification if configured via environment variables.
func GetTLSTransport(insecure bool) http.RoundTripper {
	tlsTransport, err := httptransport.TLSTransport(httptransport.TLSClientOptions{
		InsecureSkipVerify: insecure,
	})
	if err != nil {
		panic(err)
	}
	return tlsTransport
}

func IntToPointer(in int) *int32 {
	i := int32(in)
	return &i
}

func BoolToPointer(in bool) *bool {
	return &in
}

func main() {

	config := api.NewConfiguration()
	config.Debug = true
	config.Scheme = "https"
	config.Host = "172.18.255.202:9443"
	config.HTTPClient = &http.Client{
		Transport: GetTLSTransport(true),
	}

	identifier := "id"
	user := 1
	expiring := false

	tokenRequest := api.TokenRequest{
		Identifier: identifier,
		User:       IntToPointer(user),
		Expiring:   BoolToPointer(expiring),
	}

	fmt.Println(tokenRequest)

	//config.AddDefaultHeader("Authorization", fmt.Sprintf("Bearer %s", token))

	apiClient := api.NewAPIClient(config)

	//rootConfig, _, err := apiClient.RootApi.RootConfigRetrieve(context.Background()).Execute()

	resp, r, err := apiClient.AdminApi.AdminAppsList(context.Background()).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `AdminApi.AdminAppsList``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `AdminAppsList`: []App
	fmt.Fprintf(os.Stdout, "Response from `AdminApi.AdminAppsList`: %v\n", resp)
}
