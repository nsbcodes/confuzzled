import "./style.css"
import * as api from "./api.ts"

// Represents Local Piece State
// This is the client's realtime state,
// not the server's current state unless recently saved
type LocalPiece = {
	draw_x: number
	draw_y: number
	x1: number
	x2: number
	y1: number
	y2: number
	image: HTMLImageElement
	id: api.PieceID
}

// File Scope vars

let localPieces: LocalPiece[] = [] /* Holds local position data */
let canvas: HTMLCanvasElement
let ctx: CanvasRenderingContext2D
let puzzle: api.Puzzle
let userID: api.UserID
let userName: string

// Drag State
let draggedPiece: LocalPiece | null = null
let dragOffsetX = 0
let dragOffsetY = 0
let snapDistance: number
let dirtyPieces = new Set<api.PieceID>

// Polling Variables
// These are updated frequently by polling the API
let positions: api.PositionMap /* Holds server's positions */
let users: api.Users
let lockedPieces: api.LockedPieces

// Configuration Constants
const snapProportion = 0.03
const pollMS = 50
const lockedText = (name: string) => `\u{1F512} Locked by ${name}`
const lockedTextSizeProportion = 0.025

async function setLocalPieces() {
	const promises: Promise<void>[] = []

	for (let row = 0; row < puzzle.Rows; row++) {
		for (let col = 0; col < puzzle.Cols; col++) {
			const id = api.ID(row, col)
			const promise = new Promise<void>((resolve, _) => {
				let image = new Image()
				image.onload = () => {
					const cx = positions.get(id)!.X
					const cy = positions.get(id)!.Y
					let lp: LocalPiece = {
						image: image, id: id,
						draw_x: 0,
						draw_y: 0,
						x1: 0,
						x2: 0,
						y1: 0,
						y2: 0
					}
					setPiecePos(lp, cx, cy)
					localPieces.push(lp)
					resolve()
				}
				image.src = `/api/img/${id}.png`
			})
			promises.push(promise)
		}
	}

	await Promise.all(promises)
}

function drawAnchorPoint(x: number, y: number) {
	ctx.beginPath()
	ctx.arc(x, y, 20, 0, 2 * Math.PI)
	ctx.fillStyle = "#7878786e"
	ctx.fill()
	ctx.lineWidth = 4
	ctx.strokeStyle = "#4b4b4ba3"
	ctx.stroke()
}

function snapPiece(piece: LocalPiece) {
	const cx = piece.x1 + puzzle.PieceWidth / 2
	const cy = piece.y1 + puzzle.PieceHeight / 2

	for (const other of localPieces) {
		if (other === piece) continue

		const ox = other.x1 + puzzle.PieceWidth / 2
		const oy = other.y1 + puzzle.PieceHeight / 2

		for (const [tx, ty] of [
			[ox + puzzle.PieceWidth, oy],
			[ox - puzzle.PieceWidth, oy],
			[ox, oy + puzzle.PieceHeight],
			[ox, oy - puzzle.PieceHeight]
		]) {
			const dx = tx - cx
			const dy = ty - cy

			if (Math.hypot(dx, dy) <= snapDistance) {
				piece.draw_x += dx
				piece.draw_y += dy
				piece.x1 += dx
				piece.x2 += dx
				piece.y1 += dy
				piece.y2 += dy
				return
			}
		}
	}
}

function render() {
	document.getElementById("usersList")!.textContent =
		`${users.sort((a, b) => a.localeCompare(b)).join(", ")}`

	ctx.clearRect(0, 0, canvas.width, canvas.height)

	for (const p of localPieces) {
		if (p !== draggedPiece) {
			// Draw the image
			ctx.drawImage(p.image, p.draw_x, p.draw_y, p.image.width, p.image.height)

			// Draw a lock and owner if the piece is locked and it isn't ours
			if (lockedPieces.get(p.id)?.Locked && lockedPieces.get(p.id)?.UserID != userID) {
				const lockedTextSize = lockedTextSizeProportion * puzzle.CanvasHeight
				ctx.font = `normal ${lockedTextSize}px Sniglet`
				ctx.shadowColor = "transparent"
				ctx.textBaseline = "top"
				const t = lockedText(lockedPieces.get(p.id)!.Name)
				const textWidth = ctx.measureText(t).width
				const x = (p.draw_x + puzzle.Margin + puzzle.PieceWidth / 2) - (textWidth / 2)
				const y = (p.draw_y + puzzle.Margin + puzzle.PieceHeight / 2) - (lockedTextSize / 2)
				ctx.fillStyle = "#504246"
				ctx.fillRect(x, y, textWidth, lockedTextSize)
				ctx.fillStyle = "#d8e2dc"
				ctx.fillText(t, x, y)
			}
		}
	}

	if (draggedPiece) {
		// Drop shadow
		// This makes a small difference in performance
		ctx.shadowColor = "rgba(0, 0, 0, 0.2)"
		ctx.shadowBlur = 20
		ctx.shadowOffsetX = 5
		ctx.shadowOffsetY = 5
		ctx.drawImage(
			draggedPiece.image,
			draggedPiece.draw_x,
			draggedPiece.draw_y,
			draggedPiece.image.width,
			draggedPiece.image.height
		)

		// Disable shadow
		ctx.shadowColor = "transparent"

		// Now render anchor points

		// Center of dragged piece
		const cx = draggedPiece.x1 + puzzle.PieceWidth / 2
		const cy = draggedPiece.y1 + puzzle.PieceHeight / 2

		drawAnchorPoint(cx, cy)

		// Find pieces to the left, right, top and bottom
		const l = getPieceAt(cx - (puzzle.PieceWidth / 2), cy, draggedPiece)
		const r = getPieceAt(cx + (puzzle.PieceWidth / 2), cy, draggedPiece)
		const t = getPieceAt(cx, cy - (puzzle.PieceHeight / 2), draggedPiece)
		const b = getPieceAt(cx, cy + (puzzle.PieceHeight / 2), draggedPiece)

		// Draw anchors on the neighboring pieces
		if (l) drawAnchorPoint(l.x1 + (puzzle.PieceWidth / 2) + puzzle.PieceWidth, l.y1 + (puzzle.PieceHeight / 2))
		if (r) drawAnchorPoint(r.x1 + (puzzle.PieceWidth / 2) - puzzle.PieceWidth, r.y1 + (puzzle.PieceHeight / 2))
		if (t) drawAnchorPoint(t.x1 + (puzzle.PieceWidth / 2), t.y1 + (puzzle.PieceHeight / 2) + puzzle.PieceHeight)
		if (b) drawAnchorPoint(b.x1 + (puzzle.PieceWidth / 2), b.y1 + (puzzle.PieceHeight / 2) - puzzle.PieceHeight)
	}
}

function getCanvasMouse(event: PointerEvent) {
	const rect = canvas.getBoundingClientRect()

	return {
		x: (event.clientX - rect.left) * canvas.width / rect.width,
		y: (event.clientY - rect.top) * canvas.height / rect.height
	}
}

function getPieceAt(x: number, y: number, notPiece?: LocalPiece): LocalPiece | null {
	for (let i = localPieces.length - 1; i >= 0; i--) {
		const piece = localPieces[i]

		const left = piece.draw_x
		const top = piece.draw_y
		const right = left + piece.image.width
		const bottom = top + piece.image.height

		if (x >= left && x <= right && y >= top && y <= bottom && piece != notPiece) {
			return piece
		}
	}

	return null
}

function setPiecePos(piece: LocalPiece, x: number, y: number) {
	piece.draw_x = x
	piece.draw_y = y
	piece.x1 = x + puzzle.Margin
	piece.x2 = x + puzzle.Width - puzzle.Margin
	piece.y1 = y + puzzle.Margin
	piece.y2 = y + puzzle.Height - puzzle.Margin
}

function setupDragAndDrop() {
	canvas.addEventListener("pointerdown", (event: PointerEvent) => {
		const pos = getCanvasMouse(event)
		const piece = getPieceAt(pos.x, pos.y)

		if (!piece) return

		// From now on, a piece has been identified where the cursor has clicked

		draggedPiece = piece

		dragOffsetX = pos.x - piece.draw_x
		dragOffsetY = pos.y - piece.draw_y

		canvas.setPointerCapture(event.pointerId)
		canvas.style.cursor = "grabbing"

		render()
	})

	canvas.addEventListener("pointermove", (event: PointerEvent) => {
		const pos = getCanvasMouse(event)

		if (!draggedPiece) {
			// Make sure piece still exists at cursor
			const piece = getPieceAt(pos.x, pos.y)

			canvas.style.cursor = piece
				? "grab"
				: "default"
			return
		}

		// From now on, a piece has been identified where the cursor has moved

		dirtyPieces.add(draggedPiece.id)
		setPiecePos(draggedPiece, pos.x - dragOffsetX, pos.y - dragOffsetY)

		render()
	})

	async function stopDragging() {
		if (!draggedPiece) return

		// Move the dragged piece to end so it renders on top
		// after draggedPiece is set to null
		localPieces = localPieces.filter(p => p !== draggedPiece)
		localPieces.push(draggedPiece)

		snapPiece(draggedPiece)
		let _dp = draggedPiece
		draggedPiece = null
		canvas.style.cursor = "default"

		render()

		// Write position to server
		let res = await api.movePiece(userID, _dp.id, {X: _dp.draw_x, Y: _dp.draw_y})

		// Unlock the piece as well if that worked
		if (res) {
			await api.lockPiece(userID, _dp.id, false)
			dirtyPieces.delete(_dp.id)
		}
	}

	canvas.addEventListener("pointerup", stopDragging)
	canvas.addEventListener("pointercancel", stopDragging)
}

async function submitUser() {
	let nameInput = document.getElementById("submitNameInput")! as HTMLInputElement
	userName = nameInput.value.trim()
	if (userName === "") {
		window.alert("Please enter a valid username!")
		return
	}
	userID = await api.registerUser(userName)
	startPuzzle()
}

function setupNameBtn() {
	let nameBtn = document.getElementById("submitNameBtn")! as HTMLButtonElement
	let nameInput = document.getElementById("submitNameInput")! as HTMLInputElement
	nameInput.addEventListener("keydown", function (e: KeyboardEvent) {
		if (e.key === "Enter") {
			e.preventDefault()
			submitUser()
		}
	})
	nameBtn.addEventListener("click", submitUser)
}

// Polling function to get and send state
// Gets: users, lockedPieces, and positions
// Sends: locks pieces and updates positions to the server
async function synchronizeAPI() {
	// Get data
	users = await api.getUsers()
	positions = await api.getPositions()
	lockedPieces = await api.getLockedPieces()

	// Check if any pieces changed compared to server's position data
	// If so, lock the piece and then update it
	for (const piece of localPieces) {
		const serverPiecePos = positions.get(piece.id)
		
		// console.log(piece.id, piece.draw_x, serverPiecePos?.X)
		if (piece.draw_x != serverPiecePos?.X || piece.draw_y != serverPiecePos?.Y) {
			// This means that a piece ("piece") has been changed,
			// and we must update the server's state

			if (dirtyPieces.has(piece.id)) {
				// We might have moved this piece, in this case,
				// try to lock the piece and send the correct position.
				// Otherwise, if we can't lock the piece, put it back.

				let res = await api.lockPiece(userID, piece.id, true)

				if (res) {
					// Succeeded to lock piece
					// This means that the piece is ours,
					// and we should update the server's state
					await api.movePiece(userID, piece.id, { X: piece.draw_x, Y: piece.draw_y })
					dirtyPieces.delete(piece.id)
				} else {
					// Failed to lock piece
					// In this case, return it to the server's position
					setPiecePos(piece, serverPiecePos!.X, serverPiecePos!.Y)
				}
			} else {
				// This means that we did not move this piece
				// Instead, someone else did and we should update our own state
				setPiecePos(piece, serverPiecePos!.X, serverPiecePos!.Y)
			}
		}
	}

	// Rerender with updated info
	// This might trigger an unnecessary render,
	// but our interval is also not very often
	render()

	// Schedule next run of this function now that we're done
	setTimeout(synchronizeAPI, pollMS)
}


async function startPuzzle() {
	document.getElementById("input-group")!.outerHTML = `Hello ${userName}! &#x1F389<br><br>Users Joined:<span id="usersList"></span>`

	puzzle = await api.getPuzzle()
	snapDistance = puzzle.CanvasHeight * snapProportion

	document.querySelector<HTMLDivElement>('#app')!.innerHTML += `<canvas width=${puzzle.CanvasWidth} height=${puzzle.CanvasHeight} id="mainCanvas"></canvas>`
	canvas = document.getElementById("mainCanvas")! as HTMLCanvasElement
	ctx = canvas.getContext('2d')!

	await synchronizeAPI()
	await setLocalPieces()
	setupDragAndDrop()
	render()
}

async function main() {
	document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
	<div>
		<h1>Confuzzled</h1>
		<p>Procedural Puzzle Generator and Game</p>
		<div id="input-group">
			<input type="text" placeholder="Enter Your Name Here!" id="submitNameInput" />
			<input type="button" value="Submit Name" id="submitNameBtn" />
		</div>
	</div>
	`

	setupNameBtn()
}

main()
