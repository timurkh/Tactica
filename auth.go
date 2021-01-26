package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/auth"
	guuid "github.com/google/uuid"
	gorilla_context "github.com/gorilla/context"
)

type SessionData struct {
	*auth.UserRecord
	Role  string
	Admin bool
}

type SessionUtil struct {
	authClient *auth.Client
	dbUsers    UsersDatabase
}

func initSessionUtil(ac *auth.Client, db UsersDatabase) *SessionUtil {
	mdlwr := SessionUtil{
		ac, db}

	return &mdlwr
}

func (am *SessionUtil) sessionLogin(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// Get the tokens sent by the client
	idToken := r.FormValue("idToken")
	csrfToken := r.FormValue("csrfToken")

	if cookie, err := r.Cookie("csrfToken"); err == nil {
		if cookie.Value != csrfToken {
			err = errors.New("CSRF token is wrong")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return err
		}
	} else {
		err = errors.New("Failed to get CSRF cookie")
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	// Decode the IDToken
	decoded, err := am.authClient.VerifyIDToken(ctx, idToken)
	if err != nil {
		str := err.Error()
		http.Error(w, str, http.StatusUnauthorized)
		return err
	}
	time_now := time.Now().Unix()
	claimed_auth_time := int64(decoded.Claims["auth_time"].(float64))
	// Return error if the sign-in is older than 5 minutes.
	if time_now-claimed_auth_time > 5*60 {
		err = errors.New(fmt.Sprintf("Recent sign-in required, claimed_auth_time=%v, time_now=%v", time_now, claimed_auth_time))
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	// Set session expiration to 5 days.
	expiresIn := time.Hour * 24 * 5

	// Create the session cookie. This will also verify the ID token in the process.
	// The session cookie will have the same claims as the ID token.
	// To only allow session cookie setting on recent sign-in, auth_time in ID token
	// can be checked to ensure user was recently signed in before creating a session cookie.
	cookie, err := am.authClient.SessionCookie(ctx, idToken, expiresIn)
	if err != nil {
		err = errors.New("Failed to create a session cookie: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	// Set cookie policy for session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "firebaseSession",
		Value:    cookie,
		MaxAge:   int(expiresIn.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		//Secure:   true,
	})
	w.Write([]byte(`{"status": "success"}`))

	// Check if user exists in DB, add record otherwise
	userId := decoded.UID
	_, err = am.dbUsers.GetUser(ctx, userId)
	if err != nil {
		log.Println("Failed to get user " + userId + " from DB, adding new record to users collection")
		userRecord, err := am.authClient.GetUser(ctx, userId)
		if err != nil {
			return fmt.Errorf("Failed to get user record: %w", err)
		}

		userInfo := &UserInfo{
			DisplayName: userRecord.DisplayName,
			Email:       userRecord.Email,
			PhoneNumber: userRecord.PhoneNumber,
		}
		am.dbUsers.AddUser(ctx, userId, userInfo)

		if err != nil {
			return fmt.Errorf("Failed to add user to database: %w", err)
		}
	}

	return nil
}

func (am *SessionUtil) sessionLogout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name:     "firebaseSession",
		Value:    "",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   0,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
	return nil
}

func (am *SessionUtil) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sessionLogin":
		case "/login":
			cookie := http.Cookie{
				Name:  "csrfToken",
				Value: guuid.New().String(),
				//HttpOnly: true,
				//Secure:   true,
				SameSite: http.SameSiteStrictMode,
			}
			http.SetCookie(w, &cookie)
		default:
			// Get the ID token sent by the client
			cookie, err := r.Cookie("firebaseSession")
			if err != nil {
				// Session cookie is unavailable. Force user to login.
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			// Verify the session cookie. In this case an additional check is added to detect
			// if the user's Firebase session was revoked, user deleted/disabled, etc.
			decoded, err := am.authClient.VerifySessionCookieAndCheckRevoked(r.Context(), cookie.Value)
			if err != nil {
				// Session cookie is invalid. Force user to login.
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			gorilla_context.Set(r, "SessionToken", decoded)
		}
		next.ServeHTTP(w, r)
	})
}

func (am *SessionUtil) getCurrentUserID(r *http.Request) string {
	sessionToken := gorilla_context.Get(r, "SessionToken")
	return sessionToken.(*auth.Token).UID
}

func (am *SessionUtil) getCurrentUserInfo(r *http.Request) (*auth.UserRecord, error) {
	ctx := r.Context()

	return am.authClient.GetUser(ctx, am.getCurrentUserID(r))
}

func (am *SessionUtil) getSessionData(r *http.Request) *SessionData {
	u, _ := am.getCurrentUserInfo(r)

	sd := &SessionData{
		UserRecord: u,
	}

	if role, ok := u.CustomClaims["Role"]; ok {
		sd.Role = role.(string)
		sd.Admin = sd.Role == "Admin"
	}

	return sd
}
