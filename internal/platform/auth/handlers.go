package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	state := GenerateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev(),
		SameSite: http.SameSiteLaxMode,
	})

	authURL := s.oauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *AuthService) Callback(w http.ResponseWriter, r *http.Request) {
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
	token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("Token exchange error: %v", err)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token in response", http.StatusInternalServerError)
		return
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("Token verification error: %v", err)
		http.Error(w, "Failed to verify token", http.StatusUnauthorized)
		return
	}

	// Extract standard claims (provider-independent)
	var stdClaims StandardClaims
	if err := idToken.Claims(&stdClaims); err != nil {
		log.Printf("Claims parsing error: %v", err)
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	// Extract raw claims for provider-specific role extraction
	var rawClaims json.RawMessage
	if err := idToken.Claims(&rawClaims); err != nil {
		log.Printf("Raw claims parsing error: %v", err)
		http.Error(w, "Failed to parse claims", http.StatusInternalServerError)
		return
	}

	roles, err := s.roleExtractor(rawClaims, s.cfg.OIDCClientID)
	if err != nil {
		log.Printf("Role extraction error: %v", err)
		roles = nil
	}

	displayName := stdClaims.Name
	if displayName == "" {
		displayName = stdClaims.Email
	}

	userDBID, err := s.UpsertUser(ctx, stdClaims.Sub, stdClaims.Email, displayName)
	if err != nil {
		log.Printf("User upsert error: %v", err)
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	sessionID := GenerateState()
	if err := s.SetSession(ctx, sessionID, &SessionData{
		Token:       token,
		IDToken:     rawIDToken,
		UserID:      stdClaims.Sub,
		UserDBID:    userDBID,
		Email:       stdClaims.Email,
		DisplayName: displayName,
		Roles:       roles,
		Expires:     time.Now().Add(24 * time.Hour),
	}); err != nil {
		log.Printf("Session creation error: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev(),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/u/dashboard", http.StatusTemporaryRedirect)
}

// Logout handles POST /logout — clears session and redirects to OIDC provider logout (if configured).
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	var idToken string

	cookie, err := r.Cookie("session_id")
	if err == nil {
		if session, exists := s.GetSession(r.Context(), cookie.Value); exists {
			idToken = session.IDToken
		}
		s.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// If OIDC_LOGOUT_URL is set, redirect to the provider's logout endpoint.
	// Otherwise, just redirect to the home page.
	if s.cfg.OIDCLogoutURL != "" {
		logoutURL := fmt.Sprintf("%s?id_token_hint=%s&post_logout_redirect_uri=%s",
			s.cfg.OIDCLogoutURL,
			url.QueryEscape(idToken),
			url.QueryEscape(s.cfg.AppURL))
		http.Redirect(w, r, logoutURL, http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
