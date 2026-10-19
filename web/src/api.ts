export type Pos = {
    X: number
    Y: number
}

export type PositionMap = Map<PieceID, Pos>

export type Puzzle = {
    Rows: number
    Cols: number
    Width: number
    Height: number
    PieceWidth: number
    PieceHeight: number
    CanvasWidth: number
    CanvasHeight: number
    Margin: number
}

export type LockedPiece = {
    Name: string
    Locked: boolean
}

export type LockedPieces = Map<PieceID, LockedPiece>

export type UserID = string
export type PieceID = string
export type Users = string[]

export function ID(row: number, col: number): PieceID {
    return `r${row}c${col}`
}

export function registerUser(username: string): Promise<UserID> {
    return fetch(`/api/users/${encodeURIComponent(username)}`, { method: "POST" })
        .then(res => res.text())
        .then(res => { return res as UserID })
}

export function lockPiece(userID: UserID, piece: PieceID, locked: boolean): Promise<boolean> {
    return fetch(`/api/locked/${piece}`, {
        method: "PUT",
        body: JSON.stringify({
            "UserID": userID,
            "Locked": locked
        })
    }).then(res => { return res.status == 204 })
}

export function movePiece(userID: UserID, piece: PieceID, coords: Pos): Promise<boolean> {
    return fetch(`/api/locked/${piece}`, {
        method: "PUT",
        body: JSON.stringify({
            "UserID": userID,
            ...coords
        })
    }).then(res => { return res.status == 204 })
}

export function getPuzzle(): Promise<Puzzle> {
    return fetch("/api/puzzle")
        .then(res => res.json())
        .then(res => {
            return res as Puzzle
        })
}

export function getLockedPieces(): Promise<LockedPieces> {
    return fetch("/api/locked")
        .then(res => res.json())
        .then(res => {
            res = new Map(
                Object.entries(res)
            )

            return res as LockedPieces
        })
}

export function getUsers(): Promise<Users> {
    return fetch("/api/users")
        .then(res => res.json())
        .then(res => {
            return res as Users
        })
}

export function getPositions(): Promise<PositionMap> {
    return fetch("/api/position")
        .then(res => res.json())
        .then(res => {
            res = new Map(
                Object.entries(res)
            )

            return res as PositionMap
        })
}
