package main

import (
	"fmt"
	"github.com/eiannone/keyboard"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
)

var RED = "\033[38;2;255;0;0m"
var BLUE = "\033[38;2;0;0;255m"
var GREEN = "\033[38;2;0;255;0m"
var WHITE = "\033[38;2;255;255;255m"
var PURPLE = "\033[38;2;255;0;255m"
var RESET = "\033[m"

var BG_RED = "\033[48;2;255;0;0m"
var BG_BLUE = "\033[48;2;0;0;255m"
var BG_GREEN = "\033[48;2;0;255;0m"
var BG_WHITE = "\033[48;2;255;255;255m"

const (
	covered  = iota
	flagged  = iota
	revealed = iota
)

type board struct {
	rowcol      [][]glyph
	height      int
	width       int
	cursor      coord
	initialized bool
}

type glyph struct {
	color     string
	character string
	bomb      bool
	status    int
	location  coord
}

type coord struct {
	x int
	y int
}

func main() {
	board := newBoard(30, 10)
	clearScreen()
	gameLoop(board)
	fmt.Println("")
	printBoard(board, true)
}

func gameLoop(board *board) {
	gameOver := false
	printBoard(board, false)

	c := make(chan rune)

	for !gameOver {
		go getInput(c)

		char := <-c

		switch char {
		case 'q':
			gameOver = true
		case 'w':
			if board.cursor.y > 0 {
				board.cursor.y = board.cursor.y - 1
			}
		case 'a':
			if board.cursor.x > 0 {
				board.cursor.x = board.cursor.x - 1
			}
		case 's':
			board.cursor.y = board.cursor.y + 1
		case 'd':
			board.cursor.x = board.cursor.x + 1
		case 'f':
			g := getGlyph(board, board.cursor)
			if g.status == covered {
				g.status = flagged
			} else if g.status == flagged {
				g.status = covered
			}
		case 'r':
			if !board.initialized {
				initBoard(board, 15, board.cursor)
				board.initialized = true
			}
			reveal(board, board.cursor)
			if getGlyph(board, board.cursor).bomb {
				gameOver = true
			}
		}

		if checkVictory(board) {
			gameOver = true
			clearScreen()
			fmt.Println(GREEN + "YOU WIN!" + RESET)
		} else {
			go rerender(board)
		}
	}
}

func rerender(board *board) {
	clearScreen()
	printBoard(board, false)
}

func getInput(c chan rune) {
	char, key, err := keyboard.GetSingleKey()
	if err != nil {
		panic(err)
	}

	if key == keyboard.KeyEsc {
		c <- 'q'
	}

	c <- char
}

func reveal(board *board, c coord) {
	neighbors := revealGlyph(board, getGlyph(board, c))
	processed := make(map[coord]bool)

	for len(neighbors) > 0 {
		var key coord
		for k := range neighbors {
			key = k
			break
		}
		g := getGlyph(board, key)
		delete(neighbors, key)
		processed[key] = true
		newNeighbors := revealGlyph(board, g)

		//check if the new neighbors have been processed already
		for nc, nn := range newNeighbors {
			//make sure this hasn't already been processed
			val, ok := processed[nc]
			if ok && val {
				continue
			}
			neighbors[nc] = nn
		}
	}
}

func revealGlyph(board *board, g *glyph) map[coord]glyph {
	neighbors := make(map[coord]glyph)
	g.status = revealed
	if g.character != " " || g.bomb {
		return neighbors
	}

	initialNeighbors := gatherNeighbors(board, g.location)

	for n := range initialNeighbors {
		// do not continue to process neighbor bombs or numbers
		initialNeighbor := initialNeighbors[n]
		neighbors[initialNeighbor.location] = initialNeighbor
	}

	return neighbors
}

func newBoard(width int, height int) *board {
	var rc = make([][]glyph, height)
	for x := range rc {
		rc[x] = make([]glyph, width)
		for y := range rc[x] {
			rc[x][y] = glyph{color: WHITE, character: "■", bomb: false, status: covered, location: coord{x: y, y: x}}
		}
	}
	b := board{rowcol: rc, height: height, width: width, cursor: coord{x: 0, y: 0}, initialized: false}

	return &b
}

func initBoard(board *board, numMines int, start coord) {
	// add mines to the board until we're full
	mines := 0
	for mines < numMines {
		mineCoord := coord{x: rand.Intn(board.width), y: rand.Intn(board.height)}
		//check if coord is blocked
		if mineCoord.x >= start.x-1 && mineCoord.x <= start.x+1 && mineCoord.y >= start.y-1 && mineCoord.y <= start.y+1 {
			continue
		}

		if board.rowcol[mineCoord.y][mineCoord.x].bomb {
			continue
		}

		board.rowcol[mineCoord.y][mineCoord.x].bomb = true
		board.rowcol[mineCoord.y][mineCoord.x].color = RED
		mines++
	}

	//set the status of all board locations
	for row := range board.rowcol {
		for col := range board.rowcol[row] {
			if board.rowcol[row][col].bomb {
				continue
			}

			neighbors := gatherNeighbors(board, coord{x: col, y: row})

			neighborBombs := 0
			for nIdx := range neighbors {
				neighbor := neighbors[nIdx]
				if neighbor.bomb {
					neighborBombs++
				}
			}

			if neighborBombs > 0 {
				board.rowcol[row][col].character = strconv.FormatInt(int64(neighborBombs), 10)
				board.rowcol[row][col].color = BLUE
			} else {
				board.rowcol[row][col].character = " "
				board.rowcol[row][col].color = WHITE
			}
		}
	}
}

func gatherNeighbors(board *board, c coord) []glyph {
	minX := c.x - 1
	maxX := c.x + 1
	if minX < 0 {
		minX = 0
	}
	if maxX > board.width-1 {
		maxX = board.width - 1
	}

	minY := c.y - 1
	maxY := c.y + 1
	if minY < 0 {
		minY = 0
	}
	if maxY > board.height-1 {
		maxY = board.height - 1
	}

	var neighbors []glyph
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			if x == c.x && y == c.y {
				continue
			}
			neighbors = append(neighbors, board.rowcol[y][x])
		}
	}

	return neighbors
}

func checkVictory(board *board) bool {
	if !board.initialized {
		return false
	}

	for row := range board.rowcol {
		for col := range board.rowcol[row] {
			g := board.rowcol[row][col]
			if !g.bomb && g.status != revealed {
				return false
			}
		}
	}

	return true
}

func printBoard(board *board, debug bool) {
	fmt.Print(" ")
	for y := range board.rowcol[0] {
		fmt.Print(y % 10)
	}
	fmt.Print("\n")
	for row := range board.rowcol {
		fmt.Print(row % 10)
		for col := range board.rowcol[row] {
			g := board.rowcol[row][col]
			if debug {
				cha := g.character
				clr := g.color

				if g.status == flagged && g.bomb {
					clr = PURPLE
				}

				if col == board.cursor.x && row == board.cursor.y {
					clr = clr + BG_GREEN
				}

				printCharacter(clr, cha)
			} else {
				var cha string
				var clr string
				if g.status == revealed {
					cha = g.character
					clr = g.color
				} else if g.status == flagged {
					cha = "¶"
					clr = RED
				} else {
					cha = "■"
					clr = WHITE
				}
				if col == board.cursor.x && row == board.cursor.y {
					clr = clr + BG_GREEN
				}
				printCharacter(clr, cha)
			}
		}
		fmt.Print("\n")
	}
}

func printCharacter(color string, character string) {
	fmt.Print(color)
	fmt.Print(character)
	fmt.Print(RESET)
}

func getGlyph(board *board, coord coord) *glyph {
	return &board.rowcol[coord.y][coord.x]
}

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
