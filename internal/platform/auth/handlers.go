package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

func Login(w http.ResponseWriter, r *http.Request) {
	state := GenerateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   false, // Set true for production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	url := OAuth2Config.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	token, err := OAuth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusInternalServerError)
		return
	}

	idToken, err := Verifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify token", http.StatusUnauthorized)
		return
	}

	var claims OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	roles := claims.GetAllRoles(cfg.OIDCClientID)

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.Email
	}

	userDBID, err := UpsertUser(ctx, claims.Sub, claims.Email, displayName)
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	sessionID := GenerateState()
	if err := SetSession(ctx, sessionID, &SessionData{
		Token:       token,
		IDToken:     rawIDToken,
		UserID:      claims.Sub,
		UserDBID:    userDBID,
		Email:       claims.Email,
		DisplayName: displayName,
		Roles:       roles,
		Expires:     time.Now().Add(24 * time.Hour),
	}); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   false, // Set true for production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/u/dashboard", http.StatusTemporaryRedirect)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	var idToken string

	cookie, err := r.Cookie("session_id")
	if err == nil {
		if session, exists := GetSession(r.Context(), cookie.Value); exists {
			idToken = session.IDToken
		}
		DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// If OIDC_LOGOUT_URL is set, redirect to the provider's logout endpoint.
	// Otherwise, just redirect to the home page.
	if cfg.OIDCLogoutURL != "" {
		logoutURL := fmt.Sprintf("%s?id_token_hint=%s&post_logout_redirect_uri=%s",
			cfg.OIDCLogoutURL,
			url.QueryEscape(idToken),
			url.QueryEscape(cfg.AppURL))
		http.Redirect(w, r, logoutURL, http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
