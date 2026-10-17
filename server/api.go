package main

import (
	"encoding/json"
	"errors"
	"log"
	"maps"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Pos struct {
	X float64
	Y float64
}
type PositionState map[string]Pos

type LockReceive struct {
	UserID string
	Locked bool
}
type Lock struct {
	Name   string
	Locked bool
	// private - don't serialize!
	userID string
}
type LockedState map[string]Lock

// key: UUID
// value: Name (Unicode)
type Users map[string]string

// Generate puzzle pieces if the output directory does not exist
func getPuzzle() Puzzle {
	_, err := os.Stat(outputDir)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("Could not find output directory %s, generating puzzle...", outputDir)
		generatePuzzle()
	}

	// Load the puzzle.json file
	puzzleBytes, err := os.ReadFile(puzzleOutputFile)
	if err != nil {
		log.Fatalf("Could not find file %s due to error \"%s\"", puzzleOutputFile, err)
	}
	log.Printf("Successfully found puzzle at %s!", puzzleOutputFile)

	puzzle := Puzzle{}
	json.Unmarshal(puzzleBytes, &puzzle)
	return puzzle
}

// Get Position State struct instance
// Accepts number of pieces as an argument in case the state file does not exist
// Also generates the state file in this case
func (api *API) getPositionState() PositionState {
	_, err := os.Stat(stateOutputFile)
	s := make(PositionState)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("Could not find state file %s, using new state...", stateOutputFile)
		for row := range api.puzzle.Rows {
			for col := range api.puzzle.Cols {
				s[ID(row, col)] = Pos{
					X: rand.Float64() * float64(api.puzzle.Width),
					Y: rand.Float64() * float64(api.puzzle.Height),
				}
			}
		}
		api.saveState(s)
		return s
	}

	// Load the puzzle.json file
	sBytes, err := os.ReadFile(stateOutputFile)
	if err != nil {
		log.Fatalf("Could not read state output file %s due to error \"%s\"", stateOutputFile, err)
	}
	log.Printf("Using saved state found at %s!", stateOutputFile)

	json.Unmarshal(sBytes, &s)
	return s
}

// Get Locked State struct instance
// This does not persist and is just stored in memory/an individual session
func (api *API) getLockedState() LockedState {
	s := make(LockedState)
	for row := range api.puzzle.Rows {
		for col := range api.puzzle.Cols {
			s[ID(row, col)] = Lock{
				Locked: false,
			}
		}
	}
	return s
}

// Confirms API is alive
func sendHealth(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Saves positionState to stateOutputFile json file
func (api *API) saveState(s PositionState) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(stateOutputFile, b, os.ModePerm)
}

// Helper function to send JSON (bytes)
func sendJSON(w http.ResponseWriter, a any) {
	w.Header().Set("Content-Type", "application/json")

	b, err := json.Marshal(a)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Could not encode struct \"%s\" due to error \"%s\"", string(b), err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

type API struct {
	puzzle               Puzzle
	positionState        PositionState
	positionStateUpdated bool
	lockedState          LockedState
	users                Users
	mu                   sync.Mutex
}

// Get/Generate the puzzle and all other state
func InitAPI() *API {
	api := new(API)

	// Get Puzzle (if it doesn't exist, first generate it)
	api.puzzle = getPuzzle()

	// Get Position State (start with new positionState if it doesn't exist)
	api.positionState = api.getPositionState()

	// Generate new Locked State (this is stored in memory purposely)
	api.lockedState = api.getLockedState()

	// Save Updated (dirty/unsaved state) bool should be false by default
	api.positionStateUpdated = false

	// Users Map
	// When a piece is locked, client 1 sends its UUID
	// When client 2 scans for locked pieces,
	// they get the client 1's name, not their name
	api.users = make(Users)

	return api
}

func (api *API) sendPuzzle(w http.ResponseWriter) {
	sendJSON(w, api.puzzle)
}

func (api *API) sendLockedState(w http.ResponseWriter) {
	api.mu.Lock()
	defer api.mu.Unlock()
	sendJSON(w, api.lockedState)
}

func (api *API) sendPositionState(w http.ResponseWriter) {
	api.mu.Lock()
	defer api.mu.Unlock()
	sendJSON(w, api.positionState)
}

func (api *API) sendUsers(w http.ResponseWriter) {
	sendJSON(w, slices.Collect(maps.Values(api.users)))
}

func (api *API) updateLockedState(w http.ResponseWriter, r *http.Request) {
	id := path.Base(r.URL.Path)

	api.mu.Lock()
	defer api.mu.Unlock()

	piece, pieceExists := api.lockedState[id]
	if pieceExists {
		var receive LockReceive
		err := json.NewDecoder(r.Body).Decode(&receive)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		name, nameExists := api.users[receive.UserID]
		if !nameExists {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if piece.Locked && piece.userID != receive.UserID {
			w.WriteHeader(http.StatusConflict)
			return
		}

		api.lockedState[id] = Lock{Name: name, Locked: receive.Locked, userID: receive.UserID}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *API) updatePositionState(w http.ResponseWriter, r *http.Request) {
	id := path.Base(r.URL.Path)

	api.mu.Lock()
	defer api.mu.Unlock()

	_, idExists := api.positionState[id]
	if idExists {
		var pos Pos
		err := json.NewDecoder(r.Body).Decode(&pos)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		api.positionState[id] = pos
		api.positionStateUpdated = true
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *API) registerUser(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(path.Base(r.URL.Path))
	if err != nil || name == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	id := uuid.New().String()

	api.mu.Lock()
	api.users[id] = name
	api.mu.Unlock()

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(id))
}

// Host API
func (api *API) run() {
	// Returns OK and HTTP 200
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			sendHealth(w)
		}
	})

	// Register User route
	// Returns a UUID that can then be used to lock (reserve) pieces
	// /register/{name} route, returns a UUID in the response
	// name should be in encoded URI format, it can also include unicode (emojis)
	http.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			api.registerUser(w, r)
		}
	})

	// Get User route
	// Returns the values of the map as a JSON list of strings
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendUsers(w)
		}
	})

	// Returns puzzle struct
	http.HandleFunc("/puzzle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendPuzzle(w)
		}
	})

	// Returns current locked state
	http.HandleFunc("/locked", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendLockedState(w)
		}
	})

	// Sets current locked state
	// /lockedState/{ID} route, does lockedState[ID] = {Locked: newLocked}
	http.HandleFunc("/locked/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			api.updateLockedState(w, r)
		}
	})

	// Returns current position state
	http.HandleFunc("/position", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendPositionState(w)
		}
	})

	// Sets current position state
	// /position/{ID} route does positionState[ID] = {X: newX, Y: newY}
	http.HandleFunc("/position/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			api.updatePositionState(w, r)
		}
	})

	// Save State Goroutine
	// Runs every 1s if state has changed
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Only continue if we have state updated
			api.mu.Lock()
			if !api.positionStateUpdated {
				// Unlock mutex and finish
				api.mu.Unlock()
				continue
			}

			// This is a relatively long operation,
			// so cache state and unlock quickly
			s := maps.Clone(api.positionState)
			api.positionStateUpdated = false
			api.mu.Unlock()

			err := api.saveState(s)
			if err != nil {
				api.mu.Lock()
				// Retry later
				api.positionStateUpdated = true
				api.mu.Unlock()
				log.Printf("Failed to save state due to error \"%s\"", err)
			}
		}
	}()

	// Serve image static files
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("./out/img"))))

	log.Println("Puzzle API running on :8080")
	err := http.ListenAndServe(":8080", nil)
	log.Fatalf("Exiting, API failed due to error \"%s\"", err)
}
