package handler

import (
	"encoding/base64"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/rs/xid"
	"github.com/skip2/go-qrcode"

	"github.com/ngoduykhanh/wireguard-ui/enclosed"
	"github.com/ngoduykhanh/wireguard-ui/model"
	"github.com/ngoduykhanh/wireguard-ui/store"
	"github.com/ngoduykhanh/wireguard-ui/util"
)

// ShareClient handler stores the client config as a one-time Enclosed note and
// returns its link. The config is encrypted before it leaves this process.
func ShareClient(db store.IStore, share *enclosed.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !share.Enabled() {
			return c.JSON(http.StatusNotImplemented, jsonHTTPResponse{false, "Sharing is disabled: set WGUI_ENCLOSED_URL"})
		}

		var input struct {
			ID string `json:"id"`
		}
		if err := c.Bind(&input); err != nil {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Invalid request"})
		}
		if _, err := xid.FromString(input.ID); err != nil {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Please provide a valid client ID"})
		}

		clientData, err := db.GetClientByID(input.ID, model.QRCodeSettings{Enabled: false})
		if err != nil {
			return c.JSON(http.StatusNotFound, jsonHTTPResponse{false, "Client not found"})
		}
		server, err := db.GetServer()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, err.Error()})
		}
		globalSettings, err := db.GetGlobalSettings()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, err.Error()})
		}
		config := util.BuildClientConfig(*clientData.Client, server, globalSettings)

		// the text is shown in the note, the attachment imports straight into the WireGuard app
		note, err := share.Share(c.Request().Context(), config, &enclosed.File{
			Name:     clientData.Client.Name + ".conf",
			MimeType: "text/plain",
			Content:  []byte(config),
		})
		if err != nil {
			log.Errorf("Cannot share client %s config: %v", input.ID, err)
			return c.JSON(http.StatusBadGateway, jsonHTTPResponse{false, "Cannot create the share link"})
		}
		log.Infof("Shared config of client %s as a one-time link, expires %s", clientData.Client.Name, note.ExpiresAt)

		// a QR of the link lets a phone camera open it straight from the screen
		resp := map[string]interface{}{"url": note.URL, "expires_at": note.ExpiresAt}
		if png, err := qrcode.Encode(note.URL, qrcode.Medium, 256); err == nil {
			resp["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}
		return c.JSON(http.StatusOK, resp)
	}
}
