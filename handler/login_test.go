package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/ngoduykhanh/wireguard-ui/store/jsondb"
)

func TestLoginRejectsMissingFieldsWithoutPanic(t *testing.T) {
	t.Setenv("WGUI_ENDPOINT_ADDRESS", "wg.example.com")
	db, err := jsondb.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Init(); err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{
		`{}`,
		`{"username":"admin"}`,
		`{"username":"nosuchuser","password":"x"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(req, rec)

		if err := Login(db)(c); err != nil {
			t.Fatalf("%s: handler error: %v", body, err)
		}
		if rec.Code == http.StatusOK {
			t.Errorf("%s: got 200, want a rejection", body)
		}
	}
}
