package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegister_Handlers(t *testing.T) {
	type want struct {
		code int
		body string
	}
	tests := []struct {
		name string
		path string
		want want
	}{
		{name: "healthz ok", path: "/healthz", want: want{code: http.StatusOK, body: "ok"}},
		{name: "readyz ok", path: "/readyz", want: want{code: http.StatusOK, body: "ready"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			Register(mux)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.want.code {
				t.Errorf("status code mismatch\n got=%#v\nwant=%#v", rec.Code, tt.want.code)
			}
			if body := rec.Body.String(); body != tt.want.body {
				t.Errorf("body mismatch\n got=%#v\nwant=%#v", body, tt.want.body)
			}
		})
	}
}
