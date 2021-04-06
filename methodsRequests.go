package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	assist_db "assist/db"

	"github.com/gorilla/mux"
)

func (app *App) methodCreateQueue(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	squadId := params["squadId"]

	_, authLevel := app.checkAuthorization(r, "", squadId, squadAdmin|squadOwner)

	if authLevel == 0 {
		err := fmt.Errorf("Current user is not authorized to to add queues to squad " + squadId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	var qr assist_db.QueueRecord
	err := json.NewDecoder(r.Body).Decode(&qr)
	if err != nil {
		err = fmt.Errorf("Failed to decode requests queue from the HTTP request: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	qr.SquadId = squadId

	err = app.db.CreateRequestsQueue(ctx, qr.ID, &qr.QueueInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return nil
}

func (app *App) methodGetSquadQueues(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	squadId := params["squadId"]

	_, authLevel := app.checkAuthorization(r, "", squadId, squadMember|squadAdmin|squadOwner)

	if authLevel == 0 {
		err := fmt.Errorf("Current user is not authorized to get squad " + squadId + " queues")
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	queues, err := app.db.GetRequestQueues(ctx, squadId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(queues)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}

func (app *App) methodGetUserQueues(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	userId := params["userId"]

	userId, authLevel := app.checkAuthorization(r, userId, "", myself)

	if authLevel == 0 {
		err := fmt.Errorf("Current user is not authorized to get queues for user " + userId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	userData, err := app.db.GetUserData(ctx, userId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	queuesToApprove, queuesToHandle, err := app.db.GetUserRequestQueues(ctx, userData.UserTags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(struct {
		QueuesToApprove interface{} `json:"queuesToApprove"`
		QueuesToHandle  interface{} `json:"queuesToHandle"`
	}{queuesToApprove, queuesToHandle})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}
