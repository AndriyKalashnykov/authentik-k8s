package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	httptransport "github.com/go-openapi/runtime/client"
	api "goauthentik.io/api/v3"
)

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

	//identifier := "akadmin"
	//user := 1
	//expiring := false

	//tokenRequest := api.TokenRequest{
	//	Identifier: identifier,
	//	User:       IntToPointer(user),
	//	Expiring:   BoolToPointer(expiring),
	//}
	//
	//intent := api.IntentEnum(api.INTENTENUM_API)
	//tokenRequest.Intent = &intent
	//
	//fmt.Println(tokenRequest)

	token := "ak-outpost-b253971d-d4f5-4ae0-bf5a-cfe854c02462-api"
	config.AddDefaultHeader("Authorization", fmt.Sprintf("Bearer %s", token))

	apiClient := api.NewAPIClient(config)

	//rootConfig, _, err := apiClient.RootApi.RootConfigRetrieve(context.Background()).Execute()
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error when calling `AdminApi.AdminAppsList``: %v\n", err)
	//	fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", rootConfig)
	//}
	//fmt.Fprintf(os.Stdout, "Response from `AdminApi.AdminAppsList`: %v\n", rootConfig)

	list, rsp, err := apiClient.Oauth2Api.Oauth2AuthorizationCodesList(context.Background()).Execute()
	fmt.Fprintf(os.Stdout, "list`: %v\n", list)
	fmt.Fprintf(os.Stdout, "rsp`: %v\n", rsp)

	resp, r, err := apiClient.AdminApi.AdminAppsList(context.Background()).Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error when calling `AdminApi.AdminAppsList``: %v\n", err)
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
	}
	// response from `AdminAppsList`: []App
	fmt.Fprintf(os.Stdout, "Response from `AdminApi.AdminAppsList`: %v\n", resp)
}
