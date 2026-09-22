package oauth

import (
	"net/http/httptest"
	"testing"
)

func TestCallbackRejectsWrongStateAndDuplicateDelivery(t *testing.T) {
	codes := make(chan string, 1)
	failures := make(chan error, 1)
	handler := callbackHandler("expected", codes, failures)
	for _, query := range []string{"?code=stolen", "?state=wrong&code=stolen"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/callback"+query, nil))
		if response.Code != 400 {
			t.Fatalf("invalid state accepted: %d", response.Code)
		}
	}
	if len(codes) != 0 {
		t.Fatal("invalid callback delivered code")
	}
	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "/callback?state=expected&code=valid", nil))
		want := 200
		if i == 1 {
			want = 409
		}
		if response.Code != want {
			t.Fatalf("callback %d status %d", i, response.Code)
		}
	}
	if len(codes) != 1 || <-codes != "valid" {
		t.Fatal("incorrect callback delivery")
	}
}
