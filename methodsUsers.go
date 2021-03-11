package main

import (
	"assist/db"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

func (app *App) methodSetUser(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	userId := params["id"]
	// authorization check
	sd := app.sd.getCurrentUserData(r)
	if userId == "me" {
		userId = app.sd.getCurrentUserID(r)
	} else if sd.Status == db.Admin {
		// ok, admin can do that
	} else {
		// operation is not authorized, return error
		err := fmt.Errorf("Current user is not authorized to modify user %v", userId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	var user struct{ Name string }

	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	log.Printf("Updating user %v name to %v ", userId, user.Name)

	app.db.UpdateUser(ctx, userId, "DisplayName", user.Name)
	if err != nil {
		err := fmt.Errorf("Failed to update %v name: %w", userId, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	return nil
}

func (app *App) methodGetHome(w http.ResponseWriter, r *http.Request) error {

	ctx := r.Context()

	params := mux.Vars(r)
	userId := params["userId"]

	// authorization check
	userId, authLevel := app.checkAuthorization(r, userId, "", myself)
	if authLevel == 0 {
		// operation is not authorized, return error
		err := fmt.Errorf("Cannot retrieve home values for user %v", userId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	userId = app.sd.getCurrentUserID(r)
	sd := app.sd.getCurrentUserData(r)

	errs := make([]error, 3)
	var squads, pendingApprove, events interface{}
	var wg sync.WaitGroup
	wg.Add(3)

	// squads
	go func() {
		squads, errs[0] = app.db.GetSquadsCount(ctx, userId)
		wg.Done()
	}()

	// actions
	go func() {
		pendingApprove, errs[1] = app.db.GetSquadsWithPendingRequests(ctx, userId, sd.Admin)
		wg.Done()
	}()

	// events
	go func() {
		events, errs[2] = app.db.GetUserEvents(ctx, userId, 4)
		wg.Done()
	}()

	wg.Wait()

	for _, e := range errs {
		if e != nil {
			http.Error(w, e.Error(), http.StatusInternalServerError)
			return e
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(
		struct {
			Squads         interface{} `json:"squads"`
			PendingApprove interface{} `json:"pendingApprove"`
			Events         interface{} `json:"events"`
		}{squads, pendingApprove, events})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}
