package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Pos struct {
	X float64
	Y float64
}
type PosReceive struct {
	X      float64
	Y      float64
	UserID string
}
type PositionState map[string]Pos

type LockReceive struct {
	UserID string
	Locked bool
}
type Lock struct {
	Name   string
	Locked bool
	UserID string
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
		c := InitConfig()
		c.LoadConfig()
		generatePuzzle(c)
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
					X: ((rand.Float64() * 0.7) + 0.1) * float64(api.puzzle.CanvasWidth),
					Y: ((rand.Float64() * 0.7) + 0.1) * float64(api.puzzle.CanvasHeight),
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
	w.Header().Set("Cache-Control", "no-store")

	b, err := json.Marshal(a)
	if string(b) == "null" {
		b = []byte("")
	}
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
	// w.Header().Set("Access-Control-Allow-Origin", "*")
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
		if piece.Locked && piece.UserID != receive.UserID {
			w.WriteHeader(http.StatusConflict)
			return
		}

		api.lockedState[id] = Lock{Name: name, Locked: receive.Locked, UserID: receive.UserID}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *API) updatePositionState(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Access-Control-Allow-Origin", "*")
	id := path.Base(r.URL.Path)

	api.mu.Lock()
	defer api.mu.Unlock()

	_, idExists := api.positionState[id]
	if idExists {
		var pos PosReceive
		err := json.NewDecoder(r.Body).Decode(&pos)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		if pos.UserID != api.lockedState[id].UserID {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		api.positionState[id] = Pos{X: pos.X, Y: pos.Y}
		api.positionStateUpdated = true
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *API) registerUser(w http.ResponseWriter, r *http.Request) {
	// w.Header().Set("Access-Control-Allow-Origin", "*")
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

func IsDirEmpty(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// Host API
func (api *API) run() {
	// Returns OK and HTTP 200
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			sendHealth(w)
		}
	})

	// Register User route
	// Returns a UUID that can then be used to lock (reserve) pieces
	// /register/{name} route, returns a UUID in the response
	// name should be in encoded URI format, it can also include unicode (emojis)
	http.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			api.registerUser(w, r)
		}
	})

	// Get User route
	// Returns the values of the map as a JSON list of strings
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendUsers(w)
		}
	})

	// Returns puzzle struct
	http.HandleFunc("/api/puzzle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendPuzzle(w)
		}
	})

	// Returns current locked state
	http.HandleFunc("/api/locked", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendLockedState(w)
		}
	})

	// Sets current locked state
	// /lockedState/{ID} route, does lockedState[ID] = {Locked: newLocked}
	http.HandleFunc("/api/locked/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			api.updateLockedState(w, r)
		}
	})

	// Returns current position state
	http.HandleFunc("/api/position", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			api.sendPositionState(w)
		}
	})

	// Sets current position state
	// /position/{ID} route does positionState[ID] = {X: newX, Y: newY}
	http.HandleFunc("/api/position/", func(w http.ResponseWriter, r *http.Request) {
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
	http.Handle("/api/img/", http.StripPrefix("/api/img/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.FileServer(http.Dir("./out/img")).ServeHTTP(w, r)
	})))

	// Serve build output files (from "npm run build" at /web directory)
	// During development, you can skip this and use the Vite proxy to bypass CORS
	// This is already configured and "npm run dev" will enable it automatically
	stat, err := os.Stat(serveFolder)
	if err == nil && stat.IsDir() {
		empty, err := IsDirEmpty(serveFolder)
		if err == nil && !empty {
			log.Printf("Found non-empty serve folder, serving folder %s", serveFolder)

			fs := http.FileServer(http.Dir(serveFolder))
			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				// Don't serve frontend files for any API routes.
				if strings.HasPrefix(r.URL.Path, "/api/") {
					http.NotFound(w, r)
					return
				}
				// Serve existing files directly
				filePath := filepath.Join(serveFolder, filepath.Clean(r.URL.Path))
				if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
					fs.ServeHTTP(w, r)
					return
				}
				// Fall back to index.html
				http.ServeFile(w, r, filepath.Join(serveFolder, "index.html"))
			})
		} else {
			log.Println("Only serving API, failed check for serve folder emptiness.")
		}
	} else {
		log.Println("Only serving API, could not find serve folder.")
	}

	log.Println("Puzzle API running on :8080")
	err = http.ListenAndServe(":8080", nil)
	log.Fatalf("Exiting, API failed due to error \"%s\"", err)
}
