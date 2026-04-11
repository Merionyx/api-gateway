package problem

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	"github.com/gofiber/fiber/v3"
)

func TestWithCode_defaultTitles(t *testing.T) {
	t.Parallel()
	cases := []struct {
		st   int
		want string
	}{
		{http.StatusBadRequest, "Bad Request"},
		{http.StatusNotFound, "Not Found"},
		{http.StatusBadGateway, "Bad Gateway"},
		{http.StatusServiceUnavailable, "Service Unavailable"},
		{http.StatusConflict, "Conflict"},
		{http.StatusInternalServerError, "Internal Server Error"},
		{http.StatusTeapot, "Error"},
	}
	for _, tc := range cases {
		p := WithCode(tc.st, "CODE", "", "detail")
		if p.Title != tc.want {
			t.Fatalf("status %d: got title %q want %q", tc.st, p.Title, tc.want)
		}
	}
}

func TestConstructorHelpers(t *testing.T) {
	t.Parallel()
	if p := BadRequest("C", "", "d"); p.Status != http.StatusBadRequest {
		t.Fatal(p.Status)
	}
	if p := NotFound("C", "", "d"); p.Status != http.StatusNotFound {
		t.Fatal(p.Status)
	}
	if p := InternalError("C", "", "d"); p.Status != http.StatusInternalServerError {
		t.Fatal(p.Status)
	}
	if p := BadGateway("C", "", "d"); p.Status != http.StatusBadGateway {
		t.Fatal(p.Status)
	}
	if p := Conflict("C", "", "d"); p.Status != http.StatusConflict {
		t.Fatal(p.Status)
	}
	if p := ServiceUnavailable("C", "", "d"); p.Status != http.StatusServiceUnavailable {
		t.Fatal(p.Status)
	}
}

func TestRespondError_WriteInternal_WriteContractSync(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/domain", func(c fiber.Ctx) error {
		return RespondError(c, apierrors.ErrNotFound)
	})
	app.Get("/internal", func(c fiber.Ctx) error {
		return WriteInternal(c, errOpaque)
	})
	app.Get("/pipe", func(c fiber.Ctx) error {
		return WriteContractSync(c, errOpaque)
	})
	for _, path := range []string{"/domain", "/internal", "/pipe"} {
		req := httptest.NewRequest(fiber.MethodGet, path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}

var errOpaque = &testErr{}

type testErr struct{}

func (*testErr) Error() string { return "opaque" }
