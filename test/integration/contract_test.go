package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTaskContract(t *testing.T) {
	router := newAuthenticatedRouter(t)
	health := request(t, router, http.MethodGet, "/health", "")
	assertCode(t, health, 200)
	if health.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health=%s", health.Body.String())
	}
	empty := request(t, router, http.MethodGet, "/tasks", "")
	if empty.Body.String() != "[]\n" {
		t.Fatalf("empty=%s", empty.Body.String())
	}
	created := request(t, router, http.MethodPost, "/tasks", `{"title":"  task  "}`)
	assertCode(t, created, 201)
	var public map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if len(public) != 3 || public["title"] != "task" || public["done"] != false || public["id"] != float64(1) {
		t.Fatalf("changed public DTO: %s", created.Body.String())
	}
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/tasks/abc", "", 400},
		{http.MethodGet, "/tasks/999", "", 404},
		{http.MethodPatch, "/tasks/999", `{"done":true}`, 404},
		{http.MethodPost, "/tasks", `{"title":`, 400},
		{http.MethodPost, "/tasks", `{"title":" "}`, 400},
		{http.MethodPost, "/tasks", `{"titel":"typo"}`, 400},
		{http.MethodPost, "/tasks", `{"title":"a"}{"title":"b"}`, 400},
		{http.MethodPatch, "/tasks/1", `{}`, 400},
		{http.MethodPatch, "/tasks/1", `{"done":true}`, 200},
		{http.MethodPatch, "/tasks/1", `{"done":true}`, 409},
		{http.MethodPatch, "/tasks/1", `{"done":false}`, 200},
		{http.MethodPut, "/tasks/1", `{"done":true}`, 405},
	} {
		response := request(t, router, tt.method, tt.path, tt.body)
		assertCode(t, response, tt.status)
	}
}
