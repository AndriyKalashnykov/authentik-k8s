package util

import (
	"net/http"

	httptransport "github.com/go-openapi/runtime/client"
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

func StringToPointer(in string) *string {
	return &in
}

//rootConfig, _, err := apiClient.RootApi.RootConfigRetrieve(context.Background()).Execute()
//if err != nil {
//	fmt.Fprintf(os.Stderr, "Error when calling `AdminApi.AdminAppsList``: %v\n", err)
//	fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", rootConfig)
//}
//fmt.Fprintf(os.Stdout, "Response from `AdminApi.AdminAppsList`: %v\n", rootConfig)

//list, rsp, err := apiClient.Oauth2Api.Oauth2AuthorizationCodesList(context.Background()).Execute()
//if err != nil {
//	fmt.Fprintf(os.Stdout, "list`: %v\n", list)
//	fmt.Fprintf(os.Stdout, "rsp`: %v\n", rsp)
//}

//resp, r, err := apiClient.AdminApi.AdminAppsList(context.Background()).Execute()
//if err != nil {
//	fmt.Fprintf(os.Stderr, "Error when calling `AdminApi.AdminAppsList``: %v\n", err)
//	fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
//}
//// response from `AdminAppsList`: []App
//fmt.Fprintf(os.Stdout, "Response from `AdminApi.AdminAppsList`: %v\n", resp)

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
