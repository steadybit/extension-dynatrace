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
			if r.URL.Path == "/api/v2/settings/objects" && r.Method == http.MethodPost {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(maintenanceWindowCreated())
			} else if strings.HasPrefix(r.URL.Path, "/api/v2/settings/objects/") && r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			} else if r.URL.Path == "/api/v2/problems" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(problems())
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

func problems() []byte {
	return []byte(`{
    "totalCount": 1,
    "pageSize": 50,
    "problems": [
        {
            "problemId": "-703143834675302702_1701158040000V2",
            "displayId": "P-2311100",
            "title": "Container restarts",
            "impactLevel": "APPLICATION",
            "severityLevel": "ERROR",
            "status": "CLOSED",
            "affectedEntities": [
                {
                    "entityId": {
                        "id": "CLOUD_APPLICATION-7DA5F4D930A3CA81",
                        "type": "CLOUD_APPLICATION"
                    },
                    "name": "fashion-bestseller"
                }
            ],
            "impactedEntities": [
                {
                    "entityId": {
                        "id": "CLOUD_APPLICATION-7DA5F4D930A3CA81",
                        "type": "CLOUD_APPLICATION"
                    },
                    "name": "fashion-bestseller"
                }
            ],
            "rootCauseEntity": null,
            "managementZones": [],
            "entityTags": [],
            "problemFilters": [
                {
                    "id": "c21f969b-5f03-333d-83e0-4f8f136e7682",
                    "name": "Default"
                }
            ],
            "startTime": 1701158040000,
            "endTime": 1701158523979
        }
    ]
  }`)
}
