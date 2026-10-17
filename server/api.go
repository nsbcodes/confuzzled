package main

import (
	"encoding/json"
	"errors"
	"log"
	"maps"
	"math/rand/v2"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type Pos struct {
	X float64
	Y float64
}

type PositionState map[string]Pos

type Lock struct {
	Locked bool
}

type LockedState map[string]Lock

// Confirms API is alive
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

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
func getPositionState(puzzle Puzzle) PositionState {
	_, err := os.Stat(stateOutputFile)
	s := make(PositionState)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("Could not find state file %s, using new state...", stateOutputFile)
		for row := range puzzle.Rows {
			for col := range puzzle.Cols {
				s[ID(row, col)] = Pos{
					X: rand.Float64() * float64(puzzle.Width),
					Y: rand.Float64() * float64(puzzle.Height),
				}
			}
		}
		saveState(s)
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
func getLockedState(puzzle Puzzle) LockedState {
	s := make(LockedState)
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
			s[ID(row, col)] = Lock{
				Locked: false,
			}
		}
	}
	return s
}

// Saves state to stateOutputFile json file
func saveState(state PositionState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return os.WriteFile(stateOutputFile, b, os.ModePerm)
}

// Helper function to send JSON (bytes)
func sendJSON(w http.ResponseWriter, a any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	b, err := json.Marshal(a)
	if err != nil {
		log.Fatalf("Could not encode struct \"%s\" due to error \"%s\"", string(b), err)
	}

	w.Write(b)
}

// Main Function to first get/generate the puzzle and then host the API
func api() {
	// Get Puzzle (if it doesn't exist, first generate it)
	puzzle := getPuzzle()

	// Get Position State (start with new positionState if it doesn't exist)
	positionState := getPositionState(puzzle)

	// Generate new Locked State (this is stored in memory purposely)
	lockedState := getLockedState(puzzle)

	// Mutex Lock and Save Update bool
	var mu sync.Mutex
	positionStateUpdated := false

	// Host API

	// Returns OK and HTTP 200
	http.HandleFunc("/health", healthHandler)

	// Returns puzzle struct
	http.HandleFunc("/puzzle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			sendJSON(w, puzzle)
		}
	})

	// Returns current locked state
	http.HandleFunc("/locked", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			defer mu.Unlock()
			sendJSON(w, lockedState)
		}
	})

	// Sets current locked state
	// /lockedState/{ID} route, does lockedState[ID] = {Locked: newLocked}
	http.HandleFunc("/locked/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			id := path.Base(r.URL.Path)

			mu.Lock()
			defer mu.Unlock()

			_, idExists := lockedState[id]
			if idExists {
				var lock Lock
				err := json.NewDecoder(r.Body).Decode(&lock)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					return
				}
				lockedState[id] = lock
				positionStateUpdated = true
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Returns current position state
	http.HandleFunc("/position", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			defer mu.Unlock()
			sendJSON(w, positionState)
		}
	})

	// Sets current position state
	// /position/{ID} route does positionState[ID] = {X: newX, Y: newY}
	http.HandleFunc("/position/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			id := path.Base(r.URL.Path)

			mu.Lock()
			defer mu.Unlock()

			_, idExists := positionState[id]
			if idExists {
				var pos Pos
				err := json.NewDecoder(r.Body).Decode(&pos)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					return
				}
				positionState[id] = pos
				positionStateUpdated = true
			} else {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusNoContent)
		}
	})

	// Save State Goroutine
	// Runs every 1s if state has changed
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Only continue if we have state updated
			mu.Lock()
			if !positionStateUpdated {
				// Unlock mutex and finish
				mu.Unlock()
				continue
			}

			// This is a relatively long operation,
			// so cache state and unlock quickly
			s := maps.Clone(positionState)
			positionStateUpdated = false
			mu.Unlock()

			err := saveState(s)
			if err != nil {
				mu.Lock()
				// Retry later
				positionStateUpdated = true
				mu.Unlock()
				log.Printf("Failed to save state due to error \"%s\"", err)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			mu.Lock()

			if !positionStateUpdated {
				mu.Unlock()
				continue
			}

			// Make an independent snapshot of the map.
			snapshot := make(PositionState, len(positionState))
			for id, pos := range positionState {
				snapshot[id] = pos
			}

			// Clear the flag while protected by the mutex.
			positionStateUpdated = false
			mu.Unlock()

			// Disk I/O happens outside the mutex.
			if err := saveState(snapshot); err != nil {
				log.Printf("Failed to save state: %v", err)

				// Retry on the next tick.
				mu.Lock()
				positionStateUpdated = true
				mu.Unlock()
			}
		}
	}()

	// Serve image static files
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("./out/img"))))

	log.Println("Puzzle API running on :8080")
	err := http.ListenAndServe(":8080", nil)
	log.Fatalf("Exiting, API failed due to error \"%s\"", err)
}
