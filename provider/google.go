package provider

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleUser struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Profile       string `json:"profile"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Gender        string `json:"gender"`
}

func (w *Website) GetGoogleAuthConfig(focus bool) *oauth2.Config {
	if w.googleAuthConfig == nil || focus {
		setting := w.GetGoogleAuthSetting()
		if setting.ClientId == "" || setting.ClientSecret == "" {
			w.googleAuthConfig = nil
			return nil
		}

		w.googleAuthConfig = &oauth2.Config{
			ClientID:     setting.ClientId,
			ClientSecret: setting.ClientSecret,
			RedirectURL:  setting.RedirectUrl,
			Scopes: []string{
				"openid",
				"https://www.googleapis.com/auth/userinfo.email", // You have to select your own scope from here -> https://developers.google.com/identity/protocols/googlescopes#google_sign-in
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		}

		w2 := GetWebsite(w.Id)
		w2.googleAuthConfig = w.googleAuthConfig
	}

	return w.googleAuthConfig
}
