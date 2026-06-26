package util

import "testing"

func TestInt32ToPointer(t *testing.T) {
	for _, in := range []int32{0, -1, 42, 2147483647} {
		got := Int32ToPointer(in)
		if got == nil {
			t.Fatalf("Int32ToPointer(%d) = nil, want non-nil", in)
		}
		if *got != in {
			t.Errorf("*Int32ToPointer(%d) = %d, want %d", in, *got, in)
		}
	}
}

func TestBoolToPointer(t *testing.T) {
	for _, in := range []bool{true, false} {
		got := BoolToPointer(in)
		if got == nil {
			t.Fatalf("BoolToPointer(%v) = nil, want non-nil", in)
		}
		if *got != in {
			t.Errorf("*BoolToPointer(%v) = %v, want %v", in, *got, in)
		}
	}
}

func TestStringToPointer(t *testing.T) {
	for _, in := range []string{"", "x", "a longer value"} {
		got := StringToPointer(in)
		if got == nil {
			t.Fatalf("StringToPointer(%q) = nil, want non-nil", in)
		}
		if *got != in {
			t.Errorf("*StringToPointer(%q) = %q, want %q", in, *got, in)
		}
	}
}

func TestGetTLSTransport(t *testing.T) {
	for _, insecure := range []bool{true, false} {
		rt := GetTLSTransport(insecure)
		if rt == nil {
			t.Fatalf("GetTLSTransport(%v) = nil, want a non-nil http.RoundTripper", insecure)
		}
	}
}
