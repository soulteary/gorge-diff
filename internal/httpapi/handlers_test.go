package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHealthz(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("expected ok in body, got %s", rec.Body.String())
	}
}

func TestTokenAuth_NoToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "secret123"})

	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestTokenAuth_ValidToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "secret123"})

	body := `{"old":"a","new":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", "secret123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTokenAuth_QueryParam(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "secret123"})

	body := `{"old":"a","new":"a"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate?token=secret123", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTokenAuth_Disabled(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"x","new":"y"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGenerateDiff_Endpoint(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"hello\nworld\n","new":"hello\ngopher\n","oldName":"a.txt","newName":"b.txt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"equal":false`) {
		t.Fatalf("expected equal:false in response: %s", resp)
	}
	if !strings.Contains(resp, "-world") {
		t.Fatalf("expected -world in diff output: %s", resp)
	}
}

func TestProseDiff_Endpoint(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"hello world","new":"hello gopher"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/prose", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"parts"`) {
		t.Fatalf("expected parts in response: %s", resp)
	}
}

func TestGenerateDiff_BadJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestProseDiff_BadJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	req := httptest.NewRequest(http.MethodPost, "/api/diff/prose", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRootPath(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok"`) {
		t.Fatalf("expected ok in body, got %s", rec.Body.String())
	}
}

func TestTokenAuth_WrongToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "correct"})

	body := `{"old":"a","new":"b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", "wrong")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ERR_UNAUTHORIZED") {
		t.Fatalf("expected ERR_UNAUTHORIZED in body, got %s", rec.Body.String())
	}
}

func TestTokenAuth_WrongQueryParam(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "correct"})

	body := `{"old":"a","new":"b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate?token=wrong", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGenerateDiff_ResponseStructure(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"hello\n","new":"world\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"data"`) {
		t.Fatalf("expected 'data' key in response: %s", resp)
	}
	if !strings.Contains(resp, `"diff"`) {
		t.Fatalf("expected 'diff' in response data: %s", resp)
	}
	if !strings.Contains(resp, `"equal"`) {
		t.Fatalf("expected 'equal' in response data: %s", resp)
	}
}

func TestProseDiff_ResponseStructure(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"hello","new":"world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/prose", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"data"`) {
		t.Fatalf("expected 'data' key in response: %s", resp)
	}
	if !strings.Contains(resp, `"parts"`) {
		t.Fatalf("expected 'parts' in response data: %s", resp)
	}
}

func TestGenerateDiff_ErrorResponse(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"error"`) {
		t.Fatalf("expected 'error' key in response: %s", resp)
	}
	if !strings.Contains(resp, `"ERR_BAD_REQUEST"`) {
		t.Fatalf("expected ERR_BAD_REQUEST code: %s", resp)
	}
}

func TestGenerateDiff_IdenticalInput(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"same\n","new":"same\n"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"equal":true`) {
		t.Fatalf("expected equal:true for identical input: %s", rec.Body.String())
	}
}

func TestProseDiff_IdenticalInput(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{})

	body := `{"old":"same text","new":"same text"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/prose", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	resp := rec.Body.String()
	if !strings.Contains(resp, `"="`) {
		t.Fatalf("expected '=' type for identical prose: %s", resp)
	}
}

func TestTokenAuth_EmptyQueryParam(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "secret"})

	body := `{"old":"a","new":"b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/generate?token=", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestProseDiff_WithToken(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, &Deps{Token: "mytoken"})

	body := `{"old":"hello","new":"world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff/prose", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Token", "mytoken")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
