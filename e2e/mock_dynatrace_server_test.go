package e2e

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
)

var Requests []string

func createMockDynatraceServer() *httptest.Server {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		panic(fmt.Sprintf("httptest: failed to listen: %v", err))
	}
	server := httptest.Server{
		Listener: listener,
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info().Str("path", r.URL.Path).Str("method", r.Method).Str("query", r.URL.RawQuery).Msg("Request received")
			Requests = append(Requests, fmt.Sprintf("%s-%s", r.Method, r.URL.Path))
			if r.URL.Path == "/v2/settings/objects" && r.Method == http.MethodPost {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(maintenanceWindowCreated())
			} else if strings.HasPrefix(r.URL.Path, "/v2/settings/objects/") && r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		})},
	}
	server.Start()
	log.Info().Str("url", server.URL).Msg("Started Mock-Server")
	return &server
}

func maintenanceWindowCreated() []byte {
	return []byte(`[
    {
        "code": 200,
        "objectId": "MOCKED-MAINTENANCE-WINDOW-ID"
    }
]`)
}
