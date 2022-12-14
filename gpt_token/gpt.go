package gpt_token

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/satori/go.uuid"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Chat struct {
	client       *http.Client
	sessionToken string
	session      *Session
	cid          uuid.UUID
	pid          uuid.UUID
}

func (ctx *Chat) RefreshJWT() error {
	if !ctx.session.IsInvalid() {
		return nil
	}
	req, err := http.NewRequest("GET", "https://chat.openai.com/api/auth/session", nil)
	if err != nil {
		return err
	}
	req.Header.Add("User-Agent", " Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36 Edg/107.0.1418.62")
	req.Header.Add("Cookie", "__Secure-next-auth.session-token="+ctx.sessionToken)
	resp, err := ctx.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(&ctx.session); err != nil {
		return err
	}
	return nil
}

func (ctx *Chat) AutoRefreshJWT() {
	go func() {
		for {
			ctx.RefreshJWT()
			time.Sleep(time.Second)
		}
	}()
}

func (ctx *Chat) Send(text string) (*Response, error) {
	var (
		cid *uuid.UUID
		pid *uuid.UUID
	)
	if err := ctx.RefreshJWT(); err != nil {
		return nil, err
	}
	if ctx.cid != uuid.Nil {
		cid = &ctx.cid
	}
	if ctx.pid == uuid.Nil {
		tid := uuid.NewV4()
		pid = &tid
	} else {
		pid = &ctx.pid
	}
	res, err := ctx.SendMessage(text, cid, pid)
	if err != nil {
		return nil, err
	}
	ctx.cid = res.ConversationId
	ctx.pid = res.Message.ID
	return res, nil
}

func (ctx *Chat) SendMessage(text string, cid, pid *uuid.UUID) (*Response, error) {
	var chatResponse *Response
	if err := ctx.RefreshJWT(); err != nil {
		return nil, err
	}
	chatRequest := NewRequest(text, cid, pid)
	requestBytes, err := json.Marshal(chatRequest)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://chat.openai.com/backend-api/conversation", bytes.NewBuffer(requestBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", " Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/107.0.0.0 Safari/537.36 Edg/107.0.1418.62")
	req.Header.Add("Authorization", "Bearer "+ctx.session.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ctx.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	arr := strings.Split(string(responseBytes), "\n\n")
	index := len(arr) - 3
	if index >= len(arr) || index < 1 {
		return nil, errors.New(string(responseBytes))
	}
	arr = strings.Split(arr[index], "data: ")
	if len(arr) < 2 {
		return nil, errors.New(string(responseBytes))
	}
	if err = json.Unmarshal([]byte(arr[1]), &chatResponse); err != nil {
		return nil, err
	}
	return chatResponse, nil
}

func NewChat(sessionToken string) *Chat {
	chat := &Chat{client: &http.Client{}, sessionToken: sessionToken, session: &Session{}}
	chat.AutoRefreshJWT()
	return chat
}
