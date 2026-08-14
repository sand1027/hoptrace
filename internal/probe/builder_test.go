package probe

import "testing"

func TestRequestBuilder_DefaultsAndValidation(t *testing.T) {
	req, err := NewRequestBuilder().URL("https://example.com").Build()
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" {
		t.Fatalf("method=%s", req.Method)
	}

	_, err = NewRequestBuilder().URL("ftp://example.com").Build()
	if err == nil {
		t.Fatal("expected scheme error")
	}

	req, err = NewRequestBuilder().URL("example.com").Body([]byte(`{"a":1}`)).Build()
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Fatalf("expected auto POST, got %s", req.Method)
	}
	if req.URL != "https://example.com" {
		t.Fatalf("url=%s", req.URL)
	}
}

func TestRequestBuilder_SLO(t *testing.T) {
	_, err := NewRequestBuilder().URL("https://x").SLO(map[string]float64{"nope": 1}).Build()
	if err == nil {
		t.Fatal("expected unknown slo key")
	}
}
