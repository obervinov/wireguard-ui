package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/ngoduykhanh/wireguard-ui/model"
	"github.com/ngoduykhanh/wireguard-ui/sharedips"
	"github.com/ngoduykhanh/wireguard-ui/store"
	"github.com/ngoduykhanh/wireguard-ui/util"
)

type sharedAllowedIPsResponse struct {
	model.SharedAllowedIPs
	SyncEnabled bool `json:"sync_enabled"`
}

// GetSharedAllowedIPs handler returns the shared AllowedIPs list and its sync state
func GetSharedAllowedIPs(db store.IStore, syncer *sharedips.Syncer) echo.HandlerFunc {
	return func(c echo.Context) error {
		shared, err := db.GetSharedAllowedIPs()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot read shared AllowedIPs"})
		}
		return c.JSON(http.StatusOK, sharedAllowedIPsResponse{shared, syncer.Enabled()})
	}
}

// SharedAllowedIPsSubmit handler saves the static part of the shared AllowedIPs list
func SharedAllowedIPsSubmit(db store.IStore) echo.HandlerFunc {
	return func(c echo.Context) error {
		var input struct {
			StaticIPs []string `json:"static_ips"`
		}
		if err := c.Bind(&input); err != nil {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Invalid request"})
		}
		input.StaticIPs = model.MergeIPLists(input.StaticIPs)
		if !util.ValidateCIDRList(input.StaticIPs, false) {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "Shared AllowedIPs must be in CIDR format"})
		}

		shared, err := db.GetSharedAllowedIPs()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot read shared AllowedIPs"})
		}
		shared.StaticIPs = input.StaticIPs
		shared.UpdatedAt = time.Now().UTC()
		if err := db.SaveSharedAllowedIPs(shared); err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, err.Error()})
		}
		log.Infof("Updated shared AllowedIPs: %v", shared.StaticIPs)
		return c.JSON(http.StatusOK, jsonHTTPResponse{true, "Updated shared AllowedIPs successfully"})
	}
}

// SyncSharedAllowedIPs handler runs the DigitalOcean sync on demand
func SyncSharedAllowedIPs(db store.IStore, syncer *sharedips.Syncer) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !syncer.Enabled() {
			return c.JSON(http.StatusBadRequest, jsonHTTPResponse{false, "DigitalOcean sync is disabled: WGUI_DO_TOKEN is not set"})
		}
		if err := syncer.Sync(c.Request().Context()); err != nil {
			return c.JSON(http.StatusBadGateway, jsonHTTPResponse{false, err.Error()})
		}
		shared, err := db.GetSharedAllowedIPs()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, jsonHTTPResponse{false, "Cannot read shared AllowedIPs"})
		}
		return c.JSON(http.StatusOK, sharedAllowedIPsResponse{shared, true})
	}
}

// validateEffectiveAllowedIPs rejects a client that would end up with an empty AllowedIPs line
func validateEffectiveAllowedIPs(db store.IStore, client model.Client) bool {
	settings, err := db.GetGlobalSettings()
	if err != nil {
		return len(client.AllowedIPs) > 0
	}
	return len(util.EffectiveAllowedIPs(client, settings)) > 0
}
