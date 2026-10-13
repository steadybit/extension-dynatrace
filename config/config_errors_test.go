package config

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/steadybit/extension-dynatrace/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStaticServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// unreachableSpec points to a server which has already been shut down, so every request fails.
func unreachableSpec(t *testing.T) Specification {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
}

func Test_ParseConfiguration(t *testing.T) {
	previous := Config
	t.Cleanup(func() { Config = previous })

	t.Setenv("STEADYBIT_EXTENSION_API_BASE_URL", "https://abc.live.dynatrace.com/api")
	t.Setenv("STEADYBIT_EXTENSION_UI_BASE_URL", "https://abc.apps.dynatrace.com/ui")
	t.Setenv("STEADYBIT_EXTENSION_API_TOKEN", "token")
	t.Setenv("STEADYBIT_EXTENSION_INSECURE_SKIP_VERIFY", "true")

	ParseConfiguration()
	assert.NotPanics(t, ValidateConfiguration)

	assert.Equal(t, Specification{
		ApiBaseUrl:         "https://abc.live.dynatrace.com/api",
		UiBaseUrl:          "https://abc.apps.dynatrace.com/ui",
		UiProblemsPath:     "/apps/dynatrace.classic.problems/#problems/problemdetails",
		ApiToken:           "token",
		InsecureSkipVerify: true,
	}, Config)
}

func Test_do_InvalidRequest(t *testing.T) {
	s := Specification{ApiToken: "X"}
	body, resp, err := s.do("http://localhost", "INVALID METHOD", nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Nil(t, body)
}

func Test_do_IgnoresUnusableCertDirEntries(t *testing.T) {
	srv := newStaticServer(t, http.StatusOK, "ok")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not a cert"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.pem"), []byte("not a cert"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0700))
	notADir := filepath.Join(dir, "readme.txt")
	missing := filepath.Join(dir, "does-not-exist")
	t.Setenv("SSL_CERT_DIR", dir+string(os.PathListSeparator)+notADir+string(os.PathListSeparator)+missing)

	s := Specification{ApiToken: "X"}
	body, resp, err := s.do(srv.URL, http.MethodGet, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
}

func Test_PostEvent_Errors(t *testing.T) {
	t.Run("request fails", func(t *testing.T) {
		spec := unreachableSpec(t)
		res, _, err := spec.PostEvent(context.Background(), types.EventIngest{})
		assert.Error(t, err)
		assert.Nil(t, res)
	})
	t.Run("invalid response body", func(t *testing.T) {
		srv := newStaticServer(t, http.StatusCreated, "not json")
		spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
		res, resp, err := spec.PostEvent(context.Background(), types.EventIngest{})
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func Test_GetEntities_Errors(t *testing.T) {
	t.Run("request fails", func(t *testing.T) {
		spec := unreachableSpec(t)
		res, _, err := spec.GetEntities(context.Background(), "type(HOST)")
		assert.Error(t, err)
		assert.Nil(t, res)
	})
	t.Run("invalid response body", func(t *testing.T) {
		srv := newStaticServer(t, http.StatusOK, "not json")
		spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
		res, resp, err := spec.GetEntities(context.Background(), "type(HOST)")
		assert.Error(t, err)
		assert.Nil(t, res)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func Test_CreateMaintenanceWindow_Errors(t *testing.T) {
	t.Run("request fails", func(t *testing.T) {
		spec := unreachableSpec(t)
		id, _, err := spec.CreateMaintenanceWindow(context.Background(), types.CreateMaintenanceWindowRequest{})
		assert.Error(t, err)
		assert.Nil(t, id)
	})

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid response body", body: "not json"},
		{name: "object not created", body: `[{"code":400,"objectId":""}]`},
		{name: "unexpected number of objects", body: `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newStaticServer(t, http.StatusOK, tt.body)
			spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
			id, resp, err := spec.CreateMaintenanceWindow(context.Background(), types.CreateMaintenanceWindowRequest{})
			assert.Error(t, err)
			assert.Nil(t, id)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func Test_DeleteMaintenanceWindow_RequestFails(t *testing.T) {
	spec := unreachableSpec(t)
	_, err := spec.DeleteMaintenanceWindow(context.Background(), "mw-123")
	assert.Error(t, err)
}

func Test_GetProblems_Errors(t *testing.T) {
	t.Run("request fails", func(t *testing.T) {
		spec := unreachableSpec(t)
		problems, _, err := spec.GetProblems(context.Background(), time.Now(), nil)
		assert.Error(t, err)
		assert.Nil(t, problems)
	})
	t.Run("non 200 response", func(t *testing.T) {
		srv := newStaticServer(t, http.StatusUnauthorized, `{"error":"unauthorized"}`)
		spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
		problems, resp, err := spec.GetProblems(context.Background(), time.Now(), nil)
		assert.ErrorContains(t, err, "unexpected response code 401")
		assert.Nil(t, problems)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("invalid response body", func(t *testing.T) {
		srv := newStaticServer(t, http.StatusOK, "not json")
		spec := Specification{ApiBaseUrl: srv.URL, ApiToken: "X"}
		problems, _, err := spec.GetProblems(context.Background(), time.Now(), nil)
		assert.Error(t, err)
		assert.Nil(t, problems)
	})
}
