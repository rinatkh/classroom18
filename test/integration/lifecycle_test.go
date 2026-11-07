package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rinatkh/classroom18/internal/task"
)

// TestTaskLifecycle проходит тот же сценарий, который преподаватель запускает
// через curl и Insomnia: POST → POST → GET list → GET one → PATCH → DELETE.
func TestTaskLifecycle(t *testing.T) {
	router := newAuthenticatedRouter(t)

	first := request(t, router, http.MethodPost, "/tasks", `{"title":"Learn HTTP"}`)
	assertCode(t, first, http.StatusCreated)
	second := request(t, router, http.MethodPost, "/tasks", `{"title":"Write tests"}`)
	assertCode(t, second, http.StatusCreated)

	list := request(t, router, http.MethodGet, "/tasks", "")
	assertCode(t, list, http.StatusOK)
	var items []task.Task
	if err := json.NewDecoder(list.Body).Decode(&items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(items) != 2 || items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("неожиданный список: %#v", items)
	}

	one := request(t, router, http.MethodGet, "/tasks/1", "")
	assertCode(t, one, http.StatusOK)

	patched := request(t, router, http.MethodPatch, "/tasks/1", `{"done":true}`)
	assertCode(t, patched, http.StatusOK)
	if !strings.Contains(patched.Body.String(), `"done":true`) {
		t.Fatalf("PATCH не изменил done: %s", patched.Body.String())
	}

	deleted := request(t, router, http.MethodDelete, "/tasks/2", "")
	assertCode(t, deleted, http.StatusNoContent)
	if deleted.Body.Len() != 0 {
		t.Fatalf("204 содержит body: %q", deleted.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func assertCode(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status=%d, want=%d, body=%s", response.Code, want, response.Body.String())
	}
}
