package main

import (
	"fmt"
	"math/rand/v2"
)

// Represents a single puzzle piece.
// Other fields implement an ID to an undirected graph node.
// The Connector fields are intended for internal
// state for puzzle piece tab/blank generation
// When serialized with JSON, this map will be empty
// and Puzzle.pieces will resemble a set rather than a map
type Piece struct {
	topConnector    Connection
	bottomConnector Connection
	leftConnector   Connection
	rightConnector  Connection
	image           *ImageContainer
}

// Puzzle Container Object
type Puzzle struct {
	pieces           map[string]*Piece
	Rows             int
	Cols             int
	Width            int
	Height           int
	CorrectPositions map[string]Pos
}

// Puzzle Piece Connection Struct
// Contains the type of connection and data about its shape
type Connection struct {
	connectionType ConnectionType
	tabGenerator   TabGenerator
}

type ConnectionType int

// Puzzle Piece Connection Enums
// Can be Tab (sticks out), Blank (Socket), or None (flat edge)
const (
	ConnectionTab ConnectionType = iota
	ConnectionBlank
	ConnectionNone
)

// Cool helper function to invert the type enum (tab/blank)
func (c Connection) Invert() Connection {
	if c.connectionType == ConnectionBlank {
		c.connectionType = ConnectionTab
	} else if c.connectionType == ConnectionTab {
		c.connectionType = ConnectionBlank
	}
	return c
}

// Generate a random connection
func RandomConnection() Connection {
	connection := Connection{}
	connection.tabGenerator = MakeTabGenerator((rand.Float64()*0.3)+0.1, (rand.Float64()*0.4)+0.05)

	r := rand.IntN(2)
	if r == 1 {
		connection.connectionType = ConnectionTab
	} else {
		connection.connectionType = ConnectionBlank
	}
	return connection
}

// Universal puzzle piece ID helper function
func ID(row int, col int) string {
	return fmt.Sprintf("r%dc%d", row, col)
}
