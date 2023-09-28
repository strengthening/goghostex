package okex

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	. "github.com/deforceHK/goghostex"
)

type WSArgOKEx struct {
	Channel  string `json:"channel"`
	InstId   string `json:"instId,omitempty"`
	InstType string `json:"instType,omitempty"`
}

type WSOpOKEx struct {
	Op   string              `json:"op"`
	Args []map[string]string `json:"args"`
}

type WSResOKEx struct {
	Event string          `json:"event"`
	Arg   *WSArgOKEx      `json:"arg,omitempty"`
	Code  string          `json:"code,omitempty"`
	Msg   string          `json:"msg,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type WSTradeOKEx struct {
	RecvHandler  func(string)
	ErrorHandler func(error)
	Config       *APIConfig

	conn       *websocket.Conn
	subscribed []interface{}

	lastPingTS    int64
	lastRestartTS int64

	stopPingSign chan bool
	stopRecvSign chan bool
}

func (this *WSTradeOKEx) Subscribe(v interface{}) {
	this.subscribed = append(this.subscribed, v)
	if err := this.conn.WriteJSON(v); err != nil {
		this.ErrorHandler(err)
	}
}

func (this *WSTradeOKEx) Unsubscribe(v interface{}) {
	this.subscribed = append(this.subscribed, v)
	if err := this.conn.WriteJSON(v); err != nil {
		this.ErrorHandler(err)
	}
}

func (this *WSTradeOKEx) Start() {
	if this.RecvHandler == nil {
		this.RecvHandler = func(msg string) {
			log.Println(msg)
		}
	}
	if this.ErrorHandler == nil {
		this.ErrorHandler = func(err error) {
			log.Fatalln(err)
		}
	}

	var conn, _, err = websocket.DefaultDialer.Dial(
		"wss://ws.okx.com:8443/ws/v5/private",
		nil,
	)
	if err != nil {
		this.ErrorHandler(err)
		return
	}
	this.conn = conn

	var ts = fmt.Sprintf("%d", time.Now().Unix())
	var sign, _ = GetParamHmacSHA256Base64Sign(
		this.Config.ApiSecretKey,
		fmt.Sprintf("%sGET/users/self/verify", ts),
	)
	var login = WSOpOKEx{
		Op: "login",
		Args: []map[string]string{
			{
				"apiKey":     this.Config.ApiKey,
				"passphrase": this.Config.ApiPassphrase,
				"timestamp":  ts,
				"sign":       sign,
			},
		},
	}

	err = this.conn.WriteJSON(login)
	if err != nil {
		this.ErrorHandler(err)
		this.ErrorHandler(fmt.Errorf("websocket will be restart"))
		this.Restart()
		return
	}

	var messageType, p, readErr = this.conn.ReadMessage()
	if readErr != nil {
		// CloseError mean the server close the connection
		//if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		this.ErrorHandler(readErr)
		this.ErrorHandler(fmt.Errorf("websocket will be restart"))
		this.Restart()
		//} else {
		//	this.ErrorHandler(readErr)
		//}
		return
	}
	if messageType != websocket.TextMessage {
		this.ErrorHandler(fmt.Errorf("message type error"))
		return
	}

	var result = struct {
		Event string `json:"event"`
		Code  string `json:"code"`
		Msg   string `json:"msg"`
	}{}

	var jsonErr = json.Unmarshal(p, &result)
	if jsonErr != nil {
		this.ErrorHandler(jsonErr)
		return
	}
	if result.Code != "0" {
		this.ErrorHandler(fmt.Errorf("login error: %s", result.Msg))
		return
	}

	go this.pingRoutine()
	go this.recvRoutine()

}

func (this *WSTradeOKEx) pingRoutine() {
	this.stopPingSign = make(chan bool, 5)
	var ticker = time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := this.conn.WriteMessage(websocket.PingMessage,nil); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("catched ping routine!")
				}
				log.Println("error ping routine!")
				this.ErrorHandler(err)
				this.Stop()
			}
		case <-this.stopPingSign:
			return
		}
	}
}

func (this *WSTradeOKEx) recvRoutine() {
	this.stopRecvSign = make(chan bool, 5)
	var ticker = time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 超过5分钟没有收到消息，关闭连接
			if time.Now().Unix()-this.lastPingTS > 5*60 {
				this.ErrorHandler(fmt.Errorf("ping timeout"))
				this.Stop()
			}
		case <-this.stopRecvSign:
			return
		default:
			var msgType, msg, readErr = this.conn.ReadMessage()
			log.Println(msgType, readErr)
			if readErr != nil {
				this.ErrorHandler(readErr)
				//if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				this.ErrorHandler(fmt.Errorf("websocket will be restart"))
				this.Restart()
				//} else {
				//	this.Stop()
				//}
				continue
			}

			//if msgType != websocket.TextMessage {
			//	continue
			//}

			this.lastPingTS = time.Now().Unix()
			var msgStr = string(msg)
			if msgStr != "pong" {
				this.RecvHandler(msgStr)
			}
		}
	}

}

func (this *WSTradeOKEx) Stop() {
	if this.stopPingSign != nil {
		this.stopPingSign <- true
	}

	if this.stopRecvSign != nil {
		this.stopRecvSign <- true
	}

	if this.conn != nil {
		var err = this.conn.Close()
		if err != nil {
			this.ErrorHandler(err)
		}
	}

	if this.stopPingSign != nil {
		close(this.stopPingSign)
	}

	if this.stopRecvSign != nil {
		close(this.stopRecvSign)
	}

}

func (this *WSTradeOKEx) Restart() {
	this.Stop()

	var nowTS = time.Now().Unix()
	if this.lastRestartTS == 0 {
		time.Sleep(60 * time.Second)
	} else if nowTS-this.lastRestartTS < 60 {
		var waitSec = 60 - nowTS + this.lastRestartTS
		time.Sleep(time.Duration(waitSec) * time.Second)
	}
	this.lastRestartTS = time.Now().Unix()
	this.Start()

	// subscribe unsubscribe the channel
	for _, v := range this.subscribed {
		var err = this.conn.WriteJSON(v)
		if err != nil {
			this.ErrorHandler(err)
			var errmsg, _ = json.Marshal(v)
			this.ErrorHandler(fmt.Errorf("subscribe error: %s", string(errmsg)))
		}
	}

}

type WSMarketOKEx struct {
	*WSTradeOKEx
}

func (this *WSMarketOKEx) Start() {
	if this.RecvHandler == nil {
		this.RecvHandler = func(msg string) {
			log.Println(msg)
		}
	}
	if this.ErrorHandler == nil {
		this.ErrorHandler = func(err error) {
			log.Fatalln(err)
		}
	}

	var conn, _, err = websocket.DefaultDialer.Dial(
		"wss://ws.okx.com:8443/ws/v5/public",
		nil,
	)
	if err != nil {
		this.ErrorHandler(err)
		return
	}
	this.conn = conn

	go this.pingRoutine()
	go this.recvRoutine()

}
