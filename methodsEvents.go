package main

import (
	"assist/db"
	assist_db "assist/db"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	gorilla_context "github.com/gorilla/context"
	"github.com/gorilla/mux"
)

func (app *App) checkAuthorizationForEvent(r *http.Request, userId string, eventId string, requiredLevel AuthenticatedLevel) (_ string, level AuthenticatedLevel) {
	eventInfo, err := app.db.GetEvent(r.Context(), eventId)
	if err != nil {
		log.Println("Failed to get event " + eventId)
		return "", 0
	}

	return app.checkAuthorization(r, userId, eventInfo.SquadId, requiredLevel)
}

func (app *App) methodCreateEvent(w http.ResponseWriter, r *http.Request) error {

	ctx := r.Context()

	var event assist_db.EventInfo
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		err = fmt.Errorf("Failed to decode event data from the HTTP request: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	userId, authLevel := app.checkAuthorization(r, "me", event.SquadId, squadAdmin|squadOwner)
	if authLevel == 0 {
		err := fmt.Errorf("Current user is not authorized to add note to squad " + event.SquadId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	event.OwnerId = userId

	id, err := app.db.CreateEvent(ctx, &event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	// notify squad members about new event
	go func() {
		memberIds, err := app.db.GetSquadMemberIds(context.Background(), event.SquadId, []int{int(assist_db.Owner), int(assist_db.Admin), int(assist_db.Member)}, userId)
		if err != nil {
			log.Println("Failed to get list of squad " + event.SquadId + " members, will not be able to create notifications")
		}
		app.ntfs.createNotification(memberIds, "New Event", "New event '"+event.Text+"' created")
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(struct {
		ID string `json:"id"`
	}{id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

func (app *App) methodGetEvents(w http.ResponseWriter, r *http.Request) (err error) {
	params := mux.Vars(r)
	ctx := r.Context()

	userId := params["userId"]

	userId, authLevel := app.checkAuthorization(r, userId, "", myself)
	if authLevel == 0 {
		err = fmt.Errorf("Current user is not authenticated to get events for user " + userId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	v := r.URL.Query()

	var events interface{}
	if v.Get("archived") != "" {
		var timeFrom *time.Time
		from := v.Get("from")
		if from != "" {
			tf, err := time.Parse(time.RFC3339, from)
			if err != nil {
				err = fmt.Errorf("Failed to convert from to a time struct: %w", err)
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusBadRequest)
				return err
			}
			timeFrom = &tf
		}

		filter := map[string]string{
			"Status": v.Get("status"),
		}
		events, err = app.db.GetArchivedEvents(ctx, userId, timeFrom, &filter)
	} else {
		var squads []string
		squads, err = app.db.GetUserSquads(ctx, userId, "")
		if err == nil && len(squads) > 0 {
			events, err = app.db.GetEvents(ctx, squads, userId)
		}
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if events != nil {
		err = json.NewEncoder(w).Encode(events)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}
	}

	return err
}

func (app *App) methodGetEventDetails(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]

	eventInfo, err := app.db.GetEvent(r.Context(), eventId)
	if err != nil {
		err = fmt.Errorf("Failed to get event %v: %w", eventId, err)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	_, authLevel := app.checkAuthorization(r, "me", eventInfo.SquadId, squadAdmin|squadOwner)
	if authLevel == 0 {
		err = fmt.Errorf("Current user is not authenticated to get event " + eventId + " details")
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	tags, err := app.db.GetTags(ctx, eventInfo.SquadId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(struct {
		Tags  interface{} `json:"tags"`
		Event interface{} `json:"event"`
	}{tags, eventInfo})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}

func (app *App) methodGetParticipants(w http.ResponseWriter, r *http.Request) (err error) {
	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]
	v := r.URL.Query()
	from := v.Get("from")
	var timeFrom *time.Time
	if from != "" {
		tf, err := time.Parse(time.RFC3339, from)
		if err != nil {
			err = fmt.Errorf("Failed to convert from to a time struct: %w", err)
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
		timeFrom = &tf
	}

	filter := map[string]string{
		"Keys":   v.Get("keys"),
		"Status": v.Get("status"),
		"Tag":    v.Get("tag"),
	}

	eventInfo, err := app.db.GetEvent(r.Context(), eventId)
	if err != nil {
		err = fmt.Errorf("Failed to get event %v: %w", eventId, err)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	_, authLevel := app.checkAuthorization(r, "me", eventInfo.SquadId, squadAdmin|squadOwner)
	if authLevel == 0 {
		err = fmt.Errorf("Current user is not authenticated to get event " + eventId + " participants")
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	participants, err := app.db.GetParticipants(ctx, eventId, timeFrom, &filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(participants)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}

func (app *App) methodRegisterParticipant(w http.ResponseWriter, r *http.Request) error {

	params := mux.Vars(r)
	ctx := r.Context()

	userIdsString := params["userIds"]
	eventId := params["eventId"]

	userIds := strings.Split(userIdsString, ",")

	eventInfo, err := app.db.GetEvent(r.Context(), eventId)
	if err != nil {
		err := fmt.Errorf("Failed to get event %v details", eventId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	var authLevel AuthenticatedLevel
	var currentUserId string
	if len(userIds) == 1 {
		currentUserId, authLevel = app.checkAuthorization(r, userIds[0], eventInfo.SquadId, myself|squadAdmin|squadOwner)
		userIds[0] = currentUserId
	} else {
		currentUserId, authLevel = app.checkAuthorization(r, userIds[0], eventInfo.SquadId, squadAdmin|squadOwner)
	}

	var status assist_db.ParticipantStatusType
	if authLevel&(squadOwner|squadAdmin|systemAdmin) != 0 {
		status = assist_db.Going
	} else if authLevel&myself != 0 {
		status = assist_db.Applied
	} else {
		err = fmt.Errorf("Current user is not authorized to register participant " + currentUserId + " for event " + eventId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	err = app.db.RegisterParticipants(ctx, userIds, eventId, eventInfo, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	if status == assist_db.Applied {
		// notify squad admins that there is new event participant pending approve
		go func() {
			memberIds, err := app.db.GetSquadMemberIds(context.Background(), eventInfo.SquadId, []int{int(assist_db.Owner), int(assist_db.Admin)}, "")
			if err != nil {
				log.Println("Failed to get list of squad " + eventInfo.SquadId + " admins, will not be able to create notifications")
			}
			sd := app.sd.getCurrentUserData(r)
			app.ntfs.createNotification(memberIds, "New Participant", sd.DisplayName+" applied for event '"+eventInfo.Text+"'")
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(struct {
		Status assist_db.ParticipantStatusType `json:"status"`
	}{status})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return nil
}

func (app *App) methodUpdateParticipant(w http.ResponseWriter, r *http.Request) error {
	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]
	userId := params["userId"]

	var data struct {
		Status *assist_db.ParticipantStatusType `json:"status"`
	}

	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	// authorization check
	userId, authLevel := app.checkAuthorizationForEvent(r, userId, eventId, squadAdmin|squadOwner)
	if authLevel == 0 {
		// operation is not authorized, return error
		err := fmt.Errorf("Current user is not authorized to change user " + userId + " status for event " + eventId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	err = app.db.SetParticipantStatus(ctx, userId, eventId, *data.Status)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return nil
}

func (app *App) methodRemoveParticipant(w http.ResponseWriter, r *http.Request) error {

	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]
	userId := params["userId"]

	// authorization check
	userId, authLevel := app.checkAuthorizationForEvent(r, userId, eventId, myself|squadOwner|squadAdmin)
	if authLevel == 0 {
		// operation is not authorized, return error
		err := fmt.Errorf("Current user is not authorized to remove user " + userId + " from event " + eventId)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	err := app.db.DeleteParticipant(ctx, userId, eventId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return nil
}

func (app *App) methodDeleteEvent(w http.ResponseWriter, r *http.Request) error {

	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]

	// authorization check
	sd := app.sd.getCurrentUserData(r)
	if sd.Status != db.Admin {
		eventInfo, err := app.db.GetEvent(r.Context(), eventId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return err
		}
		userId := app.sd.getCurrentUserID(r)

		if eventInfo.OwnerId != userId {
			err := fmt.Errorf("Current user is not authorized to delete event " + eventId)
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return err
		}
	}
	gorilla_context.Set(r, "AuthChecked", true)

	err := app.db.DeleteEvent(ctx, eventId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return nil
}

func (app *App) methodGetCandidates(w http.ResponseWriter, r *http.Request) (err error) {
	params := mux.Vars(r)
	ctx := r.Context()

	eventId := params["eventId"]
	v := r.URL.Query()

	from := v.Get("from")
	filter := map[string]string{
		"Keys":   v.Get("keys"),
		"Status": v.Get("status"),
		"Tag":    v.Get("tag"),
	}

	eventInfo, err := app.db.GetEvent(r.Context(), eventId)
	if err != nil {
		err = fmt.Errorf("Failed to get event %v: %w", eventId, err)
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err

	}

	_, authLevel := app.checkAuthorization(r, "me", eventInfo.SquadId, squadAdmin|squadOwner)
	if authLevel == 0 {
		err = fmt.Errorf("Current user is not authenticated to get event " + eventId + " participants")
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return err
	}

	participants, err := app.db.GetCandidates(ctx, eventInfo.SquadId, eventId, from, &filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(participants)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	return err
}

func (app *App) methodArchiveEvents(w http.ResponseWriter, r *http.Request) (err error) {
	err = app.db.ArchiveOldEvents(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	gorilla_context.Set(r, "AuthChecked", true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	return nil
}
