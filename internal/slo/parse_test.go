package slo

import "testing"

func TestParseSpec(t *testing.T) {
	m, err := ParseSpec("total=500,ttfb=200")
	if err != nil {
		t.Fatal(err)
	}
	if m["total"] != 500 || m["ttfb"] != 200 {
		t.Fatalf("%v", m)
	}
	_, err = ParseSpec("total=500,total=100")
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	_, err = ParseSpec("bogus=1")
	if err == nil {
		t.Fatal("expected unknown key")
	}
}
