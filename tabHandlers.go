package main

import (
	"assist/db"
	"fmt"
	"net/http"
)

var (
	loginTmpl    = parseBodyTemplate("login.html")
	homeTmpl     = parseBodyTemplate("home.html")
	userinfoTmpl = parseBodyTemplate("userinfo.html")
	squadsTmpl   = parseBodyTemplate("squads.html")
	squadTmpl    = parseBodyTemplate("squad.html")
	eventsTmpl   = parseBodyTemplate("events.html")
	aboutTmpl    = parseBodyTemplate("about.html")
)

func (app *App) squadsHandler(w http.ResponseWriter, r *http.Request) error {

	return squadsTmpl.Execute(app, w, r, struct {
		Session *SessionData
	}{app.su.getSessionData(r)})
}

func (app *App) squadHandler(w http.ResponseWriter, r *http.Request) error {
	keys, ok := r.URL.Query()["squadId"]

	if !ok || len(keys[0]) < 1 {
		err := fmt.Errorf("Missing Squad ID")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	squadId := keys[0]

	return squadTmpl.Execute(app, w, r, struct {
		Session *SessionData
		SquadID string
	}{app.su.getSessionData(r), squadId})
}

func (app *App) eventsHandler(w http.ResponseWriter, r *http.Request) error {

	return eventsTmpl.Execute(app, w, r, struct {
		Session *SessionData
	}{app.su.getSessionData(r)})
}

func (app *App) homeHandler(w http.ResponseWriter, r *http.Request) error {

	u, _ := app.su.getCurrentUserInfo(r)
	return homeTmpl.Execute(app, w, r, struct {
		Session *SessionData
		Data    string
	}{
		app.su.getSessionData(r),
		fmt.Sprintf("%+v<br>%+v", u, u.ProviderUserInfo[0])})
}

func (app *App) aboutHandler(w http.ResponseWriter, r *http.Request) error {

	if app.su.getCurrentUserID(r) != "" {
		return aboutTmpl.Execute(app, w, r, struct {
			Session *SessionData
		}{
			app.su.getSessionData(r),
		})
	} else {
		return aboutTmpl.Execute(app, w, r, nil)
	}
}

func (app *App) userinfoHandler(w http.ResponseWriter, r *http.Request) error {

	ctx := r.Context()
	sessionData := app.su.getSessionData(r)
	user, _ := app.db.GetUser(ctx, sessionData.UID)

	return userinfoTmpl.Execute(app, w, r, struct {
		Session *SessionData
		Data    *db.UserInfo
	}{sessionData, user})
}

func (app *App) loginHandler(w http.ResponseWriter, r *http.Request) error {
	return loginTmpl.Execute(app, w, r, nil)
}
