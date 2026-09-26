package fund

import (
	"os"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// applyEnvConfig copies deployment settings from environment variables into PocketBase
// on every start, so the campus server is configured with an env file alone.
// Anything left unset keeps whatever was configured in the PocketBase dashboard (/_/).
//
//	APP_URL                public URL of the site (Google redirects back to APP_URL/auth/callback)
//	GOOGLE_CLIENT_ID       "Continue with Google"
//	GOOGLE_CLIENT_SECRET
//	TRUSTED_PROXY_HEADERS  e.g. X-Forwarded-For behind Apache, so logs and rate limits see real client IPs
func applyEnvConfig(app core.App) error {
	env := os.Getenv
	settings := app.Settings()
	changed := false
	if v := env("APP_URL"); v != "" && settings.Meta.AppURL != v {
		settings.Meta.AppURL, changed = v, true
	}
	if settings.Meta.AppName != "Auxesis Capital" {
		settings.Meta.AppName, changed = "Auxesis Capital", true
	}
	if v := env("TRUSTED_PROXY_HEADERS"); v != "" && strings.Join(settings.TrustedProxy.Headers, ",") != v {
		settings.TrustedProxy.Headers = strings.Split(v, ",")
		settings.TrustedProxy.UseLeftmostIP = false // the proxy appends the real client IP on the right
		changed = true
	}
	if changed {
		if err := app.Save(settings); err != nil {
			return err
		}
	}

	id, secret := env("GOOGLE_CLIENT_ID"), env("GOOGLE_CLIENT_SECRET")
	if id == "" || secret == "" {
		return nil
	}
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	cfg := core.OAuth2ProviderConfig{Name: "google", ClientId: id, ClientSecret: secret}
	found := false
	for i, p := range users.OAuth2.Providers {
		if p.Name == "google" {
			if p.ClientId == id && p.ClientSecret == secret {
				return nil
			}
			users.OAuth2.Providers[i], found = cfg, true
		}
	}
	if !found {
		users.OAuth2.Providers = append(users.OAuth2.Providers, cfg)
	}
	users.OAuth2.Enabled = true
	return app.Save(users)
}
