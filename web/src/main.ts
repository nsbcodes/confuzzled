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
let users: api.Users
let lockedPieces: api.LockedPieces

// Drag State
let draggedPiece: LocalPiece | null = null
let dragOffsetX = 0
let dragOffsetY = 0
let snapDistance: number

// Polling Variables
// These are updated frequently by polling the API
let positions: api.PositionMap /* Holds server's positions */

// Configuration Constants
const snapProportion = 0.03
const pollMs = 100

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
					const lp: LocalPiece = {
						draw_x: cx - puzzle.Margin,
						draw_y: cy - puzzle.Margin,
						x1: cx,
						x2: cx + puzzle.PieceWidth,
						y1: cy,
						y2: cy + puzzle.PieceHeight,
						image: image,
						id: id
					}
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
	ctx.clearRect(0, 0, canvas.width, canvas.height)

	for (const p of localPieces) {
		if (p !== draggedPiece) {
			// Draw the image
			ctx.drawImage(p.image, p.draw_x, p.draw_y, p.image.width, p.image.height)

			// Disable shadow
			ctx.shadowColor = "transparent"
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

function setupDragAndDrop() {
	canvas.addEventListener("pointerdown", (event: PointerEvent) => {
		const pos = getCanvasMouse(event)
		const piece = getPieceAt(pos.x, pos.y)

		if (!piece) return

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
			// Make sure piece exists at cursor
			const piece = getPieceAt(pos.x, pos.y)

			canvas.style.cursor = piece
				? "grab"
				: "default"
			return
		}

		draggedPiece.draw_x = pos.x - dragOffsetX
		draggedPiece.draw_y = pos.y - dragOffsetY
		draggedPiece.x1 = draggedPiece.draw_x + puzzle.Margin
		draggedPiece.x2 = draggedPiece.draw_x + puzzle.Width - puzzle.Margin
		draggedPiece.y1 = draggedPiece.draw_y + puzzle.Margin
		draggedPiece.y2 = draggedPiece.draw_y + puzzle.Height - puzzle.Margin

		render()
	})

	function stopDragging() {
		if (!draggedPiece) return

		// Move the dragged piece to end so it renders on top
		localPieces = localPieces.filter(p => p !== draggedPiece)
		localPieces.push(draggedPiece)

		snapPiece(draggedPiece)
		draggedPiece = null
		canvas.style.cursor = "default"
		render()
	}

	canvas.addEventListener("pointerup", stopDragging)
	canvas.addEventListener("pointercancel", stopDragging)
}

function setupNameBtn() {
	let nameBtn = document.getElementById("submitNameBtn")! as HTMLButtonElement
	let nameInput = document.getElementById("submitNameInput")! as HTMLInputElement
	nameBtn.addEventListener("click", async () => {
		userName = nameInput.value.trim()
		if (userName === "") {
			window.alert("Please enter a valid username!")
			return
		}
		userID = await api.registerUser(userName)
		startPuzzle()
	})
}

// Polling function to get and send state
async function synchronizeAPI() {
	users = await api.getUsers()
	lockedPieces = await api.getLockedPieces()
	positions = await api.getPositions()

	// Schedule next run of this function after this one has been completed
	setTimeout(synchronizeAPI, pollMs)
}


async function startPuzzle() {
	document.getElementById("input-group")!.outerHTML = `Hello ${userName}! &#x1F389`

	puzzle = await api.getPuzzle()
	snapDistance = puzzle.CanvasHeight * snapProportion
	positions = await api.getPositions()

	document.querySelector<HTMLDivElement>('#app')!.innerHTML += `<canvas width=${puzzle.CanvasWidth} height=${puzzle.CanvasHeight} id="mainCanvas"></canvas>`
	canvas = document.getElementById("mainCanvas")! as HTMLCanvasElement
	ctx = canvas.getContext('2d')!

	await setLocalPieces()
	setupDragAndDrop()
	render()
}

async function main() {
	document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
	<div>
		<h1>Confuzzled</h1>
		<p>Procedural Puzzle Generator</p>
		<div id="input-group">
			<input type="text" placeholder="Enter Your Name Here!" id="submitNameInput" />
			<input type="button" value="Submit Name" id="submitNameBtn" />
		</div>
	</div>
	`

	setupNameBtn()
}

main()
