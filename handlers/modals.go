package handlers

import (
	"database/sql"
	"encoding/hex"
	"eth2-exporter/db"
	"eth2-exporter/types"
	"eth2-exporter/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// UsersModalAddValidator a validator to the watchlist and subscribes to events
func UsersModalAddValidator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := getUser(r)

	err := r.ParseForm()
	if err != nil {
		utils.LogError(err, "error parsing form", 0)
		utils.SetFlash(w, r, authSessionName, "Error: Something went wrong, please try again in a bit.")
		http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
		return
	}

	validatorForm := r.FormValue("validator")

	validators := []string{}
	invalidValidators := []string{}
	for _, userInput := range strings.Split(validatorForm, ",") {
		if utils.IsValidEnsDomain(userInput) || utils.IsEth1Address(userInput) {
			searchResult, err := FindValidatorIndicesByEth1Address(userInput)
			if err != nil {
				invalidValidators = append(invalidValidators, userInput)
				continue
			}
			for _, res := range searchResult {
				for _, index := range res.ValidatorIndices {
					validators = append(validators, fmt.Sprintf("%v", index))
				}
			}
		} else if _, err := strconv.ParseUint(userInput, 10, 32); err == nil {
			validators = append(validators, userInput)
		} else {
			invalidValidators = append(invalidValidators, userInput)
		}
	}
	if len(invalidValidators) > 0 {
		desc := "validator"
		if len(invalidValidators) > 1 {
			desc = "validators"
		}
		logger.Warn("Invalid validators when adding to watchlist: ", invalidValidators)
		utils.SetFlash(w, r, authSessionName, fmt.Sprintf("Error: Invalid %s %v. No validators added to the watchlist, please try again in a bit.", desc, strings.Join(invalidValidators, ", ")))
		http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
		return
	}

	errorMsg := "Error: Something went wrong. No validators added to the watchlist, please try again in a bit."

	pubkeys := make([][]byte, len(validators))
	for i, val := range validators {
		pubkey, _, err := GetValidatorIndexFrom(val)
		if err != nil {
			if err == sql.ErrNoRows {
				logger.Warnf("Could not find validator %v when trying to add to watchlist", val)
				utils.SetFlash(w, r, authSessionName, fmt.Sprintf("Error: Could not find validator %v. No validators added to the watchlist, please try again.", val))
			} else {
				utils.LogError(err, "error parsing form", 0)
				utils.SetFlash(w, r, authSessionName, errorMsg)
			}
			http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
			return
		}
		pubkeys[i] = pubkey
	}

	for _, pubkey := range pubkeys {
		err = db.AddToWatchlist([]db.WatchlistEntry{{UserId: user.UserID, Validator_publickey: hex.EncodeToString(pubkey)}}, utils.GetNetwork())
		if err != nil {
			logger.WithError(err).Errorf("error adding validator to watchlist: %v", user.UserID)
			utils.SetFlash(w, r, authSessionName, errorMsg)
			http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
			return
		}

		for _, ev := range types.AddWatchlistEvents {
			if r.FormValue(string(ev.Event)) == "on" {
				err := db.AddSubscription(user.UserID, utils.GetNetwork(), ev.Event, hex.EncodeToString(pubkey), 0)
				if err != nil {
					logger.WithError(err).Errorf("error adding subscription for user: %v", user.UserID)
					utils.SetFlash(w, r, authSessionName, errorMsg)
					http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
					return
				}
			} else {
				err := db.DeleteSubscription(user.UserID, utils.GetNetwork(), ev.Event, hex.EncodeToString(pubkey))
				if err != nil {
					logger.WithError(err).Errorf("error deleting subscription for user: %v", user.UserID)
					utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating a subscription, please try again in a bit.")
					http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
					return
				}
			}
		}
	}

	http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
}

// UserModalAddNetworkEvent subscribes the user for a network notification
func UserModalAddNetworkEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := getUser(r)

	err := r.ParseForm()
	if err != nil {
		utils.LogError(err, "error parsing form", 0)
		utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating your network subscriptions, please try again in a bit.")
		http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
		return
	}

	for _, ev := range types.NetworkNotificationEvents {
		if r.FormValue(string(ev.Event)) == "on" || r.FormValue("all") == "on" {
			err := db.AddSubscription(user.UserID, utils.GetNetwork(), ev.Event, string(ev.Event), 0)
			if err != nil {
				logger.WithError(err).Errorf("error adding subscription for user: %v", user.UserID)
				utils.SetFlash(w, r, authSessionName, "Error: Something went wrong adding a network subscription, please try again in a bit.")
				http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
				return
			}
		} else {
			err := db.DeleteSubscription(user.UserID, utils.GetNetwork(), ev.Event, string(ev.Event))
			if err != nil {
				logger.WithError(err).Errorf("error deleting subscription for user: %v", user.UserID)
				utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating a network subscription, please try again in a bit.")
				http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
				return
			}
		}
	}

	http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
}

// UserModalRemoveSelectedValidator a validator to the watchlist and subscribes to events
// Takes the POST of a form with an input field attr name = validators and value = <comam separated list of validator pubkeys>
func UserModalRemoveSelectedValidator(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := getUser(r)

	err := r.ParseForm()
	if err != nil {
		utils.LogError(err, "error parsing form", 0)
		utils.SetFlash(w, r, authSessionName, "Error: Something went wrong removing your validators from the watchlist, please try again in a bit.")
		http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
		return
	}

	validatorsInput := r.FormValue("validators")
	validators := strings.Split(validatorsInput, ",")

	hasError := false
	for _, v := range validators {
		err := db.RemoveFromWatchlist(user.UserID, v, utils.GetNetwork())
		if err != nil {
			logger.WithError(err).Errorf("error removing validator from watchlist")
			if !hasError {
				utils.SetFlash(w, r, authSessionName, "Error: Could not remove one or more of your validators.")
				hasError = true
			}
		}
	}

	http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
}

// UserModalManageNotificationModal a validator to the watchlist and subscribes to events
func UserModalManageNotificationModal(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	user := getUser(r)

	err := r.ParseForm()
	if err != nil {
		utils.LogError(err, "error parsing form", 0)
		utils.SetFlash(w, r, authSessionName, "Error: Something went wrong adding your validator to the watchlist, please try again in a bit.")
		http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
		return
	}

	// const VALIDATOR_EVENTS = ['validator_attestation_missed', 'validator_proposal_missed', 'validator_proposal_submitted', 'validator_got_slashed', 'validator_synccommittee_soon']
	// const MONITORING_EVENTS = ['monitoring_machine_offline', 'monitoring_hdd_almostfull', 'monitoring_cpu_load']

	validatorsForm := r.FormValue("validators")

	validators := strings.Split(validatorsForm, ",")

	events := make(map[types.EventName]bool, 0)
	for _, ev := range types.AddWatchlistEvents {
		events[ev.Event] = r.FormValue(string(ev.Event)) == "on"
	}

	for _, validator := range validators {
		pubkey, _, err := GetValidatorIndexFrom(validator)
		if err != nil {
			utils.LogError(err, "error parsing form", 0)
			utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating the validators in your watchlist, please try again in a bit.")
			http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
			return
		}

		for eventName, active := range events {
			if active {
				err := db.AddSubscription(user.UserID, utils.GetNetwork(), eventName, hex.EncodeToString(pubkey), 0)
				if err != nil {
					logger.WithError(err).Errorf("error adding subscription for user: %v", user.UserID)
					utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating the validators in your watchlist, please try again in a bit.")
					http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
					return
				}
			} else {
				err := db.DeleteSubscription(user.UserID, utils.GetNetwork(), eventName, hex.EncodeToString(pubkey))
				if err != nil {
					logger.WithError(err).Errorf("error deleting subscription for user: %v", user.UserID)
					utils.SetFlash(w, r, authSessionName, "Error: Something went wrong updating the validators in your watchlist, please try again in a bit.")
					http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
					return
				}
			}
		}
	}

	http.Redirect(w, r, "/user/notifications", http.StatusSeeOther)
}
