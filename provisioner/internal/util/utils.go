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

func Int32ToPointer(in int32) *int32 {
	i := int32(in)
	return &i
}

func BoolToPointer(in bool) *bool {
	return &in
}

func StringToPointer(in string) *string {
	return &in
}
