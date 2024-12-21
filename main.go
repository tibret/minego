package main

import (
	"minego/minego"

	"encoding/json"
	"fmt"
	"io"
	"minego/twitch"
	"net/http"
	"os"

	// "github.com/coder/websocket"
	"github.com/eiannone/keyboard"
)

func main() {

	if len(os.Args) > 1 && os.Args[1] == "keyboard" {
		runGameKeyboard()
	} else {
		http.HandleFunc("/", getRoot)
		http.HandleFunc("/startGame", startGame)

		err := http.ListenAndServe(":3000", nil)
		if err != nil {
			panic(err)
		}
	}
}

func runGameKeyboard() {
	keyboardInput := KeyboardInputProvider{}
	board := minego.NewBoard(30, 10)
	minego.ClearScreen()
	minego.GameLoop(board, keyboardInput)
	minego.PrintBoard(board, true)
}

func getRoot(res http.ResponseWriter, req *http.Request) {
	url := "https://id.twitch.tv/oauth2/authorize?client_id=" + twitch.ClientID + "&redirect_uri=http%3A%2F%2Flocalhost%3A3000%2FstartGame&response_type=code&scope=channel%3Abot%20user%3Aread%3Achat%20user%3Abot"
	io.WriteString(res, "<html><body><a href=\""+url+"\">Click here to start game</a></body></html>")
}

func startGame(res http.ResponseWriter, req *http.Request) {
	fmt.Println("Starting game...")
	params := req.URL.Query()

	code := params.Get("code")
	fmt.Println("Using Token", code)

	authToken := twitch.Auth(code)
	fmt.Println("Got Auth Token", authToken)

	setupWebsocket(authToken)
}

type KeyboardInputProvider struct {
	minego.InputProvider
}

func (k KeyboardInputProvider) GetInput(c chan rune) {
	char, key, err := keyboard.GetSingleKey()
	if err != nil {
		panic(err)
	}

	if key == keyboard.KeyEsc {
		c <- 'q'
	}

	c <- char
}

func setupWebsocket(authToken string) {
	conn := twitch.NewConnection()
	defer conn.Cancel()
	twitch.SubscribeToEvent(conn, "channel.chat.message", authToken)

	// twitchInputProvider := twitch.TwitchInputProvider{TwitchConn: conn}

	board := minego.NewBoard(30, 10)
	minego.ClearScreen()
	minego.PrintBoard(board, false)
	gameOver := false

	for !gameOver {
		_, data, err := conn.Conn.Read(conn.Context)
		if err != nil {
			panic(err)
		}

		// process metadata
		var metadata twitch.MessageMetadata
		err = json.Unmarshal(data, &metadata)
		if err != nil {
			break
		}

		if metadata.Metadata.MessageType == twitch.KeepAlive {
			continue
		} else if metadata.Metadata.MessageType == twitch.Notification {
			payload := twitch.GetMessageText(data)
			// fmt.Println(metadata.Metadata.MessageType, metadata.Metadata.MessageId, payload)
			if len(payload) == 1 {
				runes := []rune(payload)
				gameOver = minego.SendMove(board, runes[0])
			}
		}
	}

	minego.PrintBoard(board, true)

	fmt.Println("Game ended, connection closed")
}
