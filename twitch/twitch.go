package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"minego/secrets"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
)

const (
	twitchWebsocketUrl = "wss://eventsub.wss.twitch.tv/ws"
	twitchUserUrl      = "https://api.twitch.tv/helix/users?login=tibrets"
	twitchEventSubUrl  = "https://api.twitch.tv/helix/eventsub/subscriptions"
	twitchAuthUrl      = "https://id.twitch.tv/oauth2/token"
	broadcasterId      = "broadcaster_user_id"
	KeepAlive          = "session_keepalive"
	Notification       = "notification"
)

type ConnectionMessage struct {
	Payload struct {
		Session struct {
			Id     string `json:"id"`
			Status string `json:"status"`
		} `json:"session"`
	} `json:"payload"`
}

type WebsocketSubscriptionMessage struct {
	Type      string    `json:"type"`
	Version   string    `json:"version"`
	Transport Transport `json:"transport"`
	Condition Condition `json:"condition"`
}

type Transport struct {
	Method    string `json:"method"`
	SessionId string `json:"session_id"`
}

type Condition struct {
	BroadcasterId string `json:"broadcaster_user_id"`
	UserId        string `json:"user_id"`
}

type UserInfos struct {
	UserInfo []UserInfo `json:"data"`
}

type UserInfo struct {
	Id string `json:"id"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type TwitchConnection struct {
	Conn      *websocket.Conn
	Context   context.Context
	SessionId string
	Cancel    context.CancelFunc
}

type MessageMetadata struct {
	Metadata struct {
		MessageId   string `json:"message_id"`
		MessageType string `json:"message_type"`
	} `json:"metadata"`
}

type MessagePayload struct {
	Payload struct {
		Event struct {
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
		} `json:"event"`
	} `json:"payload"`
}

func NewConnection() *TwitchConnection {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	c, _, err := websocket.Dial(ctx, twitchWebsocketUrl, nil)
	if err != nil {
		panic(err)
	}

	msgType, data, err := c.Read(ctx)
	if err != nil {
		panic(err)
	}

	var msg ConnectionMessage
	err = json.Unmarshal(data, &msg)
	if err != nil {
		panic(err)
	}

	fmt.Print(msgType.String())
	fmt.Print(": ")
	fmt.Print(string(data))
	fmt.Print("\n")
	fmt.Print("Session ID:", msg.Payload.Session.Id, "\n\n")

	return &TwitchConnection{Conn: c, Context: ctx, SessionId: msg.Payload.Session.Id, Cancel: cancel}
}

func Auth(code string) string {
	res, err := http.PostForm(twitchAuthUrl, url.Values{"client_id": {secrets.ClientID}, "client_secret": {secrets.ClientSecret}, "code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {"http://localhost:3000/startGame"}})
	if err != nil {
		panic(err)
	}

	var authRes AuthResponse
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(resBody, &authRes)

	return authRes.AccessToken
}

func SubscribeToEvent(conn *TwitchConnection, eventType string, authToken string) {
	msg := newWebsocketSubscriptionMessage(eventType, conn.SessionId, authToken)
	data, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	fmt.Println("Sending Data", string(data))

	req, err := http.NewRequest(http.MethodPost, twitchEventSubUrl, bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Authorization", "Bearer "+authToken)
	req.Header.Add("Client-Id", secrets.ClientID)
	req.Header.Add("Content-Type", "application/json")

	client := http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		panic(err)
	}

	fmt.Println("Subscription to", eventType, ":", res.Status)
	resBody, _ := io.ReadAll(res.Body)
	fmt.Println(string(resBody))
}

func newWebsocketSubscriptionMessage(subscriptionType string, sessionId string, authToken string) *WebsocketSubscriptionMessage {
	message := WebsocketSubscriptionMessage{Version: "1", Transport: Transport{Method: "websocket"}, Condition: Condition{}}
	message.Type = subscriptionType
	message.Transport.SessionId = sessionId

	client := http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, twitchUserUrl, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Add("Authorization", "Bearer "+authToken)
	req.Header.Add("Client-Id", secrets.ClientID)
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println("User Response", string(resBody))

	var userInfos UserInfos
	err = json.Unmarshal(resBody, &userInfos)
	if err != nil {
		panic(err)
	}

	message.Condition.BroadcasterId = userInfos.UserInfo[0].Id
	message.Condition.UserId = userInfos.UserInfo[0].Id

	return &message
}

type TwitchInputProvider struct {
	TwitchConn *TwitchConnection
}

func (t TwitchInputProvider) GetInput(c chan rune) {
	_, data, err := t.TwitchConn.Conn.Read(t.TwitchConn.Context)
	if err != nil {
		panic(err)
	}

	// process metadata
	var metadata MessageMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		panic(err)
	}
	if metadata.Metadata.MessageType == KeepAlive {
		return
	} else if metadata.Metadata.MessageType == Notification {
		payload := GetMessageText(data)
		fmt.Println(metadata.Metadata.MessageType, metadata.Metadata.MessageId, payload)
		if len(payload) == 1 {
			runes := []rune(payload)
			c <- runes[0]
		}
	}

	c <- 'n'
}

func GetMessageText(data []byte) string {
	var payload MessagePayload
	err := json.Unmarshal(data, &payload)
	if err != nil {
		panic(err)
	}

	return payload.Payload.Event.Message.Text
}
