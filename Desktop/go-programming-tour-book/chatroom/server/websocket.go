package server

import (
	"chatroom/logic"
	"log"
	"net/http"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func WebSocketHandleFunc(w http.ResponseWriter, req *http.Request) {
	// Accept 从客户端接收WebSocket握手, 并将连接升级到WebSocket
	// 如果Origin域和主机不同, Accept将拒绝握手, 除非设置了InsecureSkipVerify选项(通过第三个参数AcceptOption设置)
	// 默认情况下, 它不允许跨源请求; 如果发生错误, Accept 将始终写入适当的响应
	conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		log.Println("websocket accept error: ", err)
		return
	}

	// 1. 新用户进来, 构建该用户的实例
	token := req.FormValue("token")
	nickname := req.FormValue("nickname")
	if l := len(nickname); l < 2 || l > 20 {
		log.Println("nickname illegal: ", nickname)
		wsjson.Write(req.Context(), conn, logic.NewErrorMessage("非法昵称, 昵称长度: 4-20"))
		conn.Close(websocket.StatusUnsupportedData, "nickname illegal!")
		return
	}
	if !logic.Broadcaster.CanEnterRoom(nickname) {
		log.Println("昵称已经存在: ", nickname)
		wsjson.Write(req.Context(), conn, logic.NewErrorMessage("该昵称已经存在!"))
		conn.Close(websocket.StatusUnsupportedData, "nickname exists!")
		return
	}

	userHasToken := logic.NewUser(conn, token, nickname, req.RemoteAddr)

	// 2. 开启给用户发送消息的goroutine
	go userHasToken.SendMessage(req.Context())

	// 3. 给当前用户发送欢迎消息
	userHasToken.MessageChannel <- logic.NewWelcomeMessage(userHasToken)

	// 避免token泄露
	tmpUser := *userHasToken
	user := &tmpUser
	user.Token = ""

	// 给所有用户告知新用户到来
	msg := logic.NewUserEnterMessage(user)
	logic.Broadcaster.Broadcast(msg)

	// 4. 将该用户加入到广播器的用户列表中
	logic.Broadcaster.UserEntering(user)
	log.Println("user: ", nickname, "joins chat")

	// 5. 接收用户消息
	err = user.ReceiveMessage(req.Context())

	// 6. 用户离开
	logic.Broadcaster.UserLeaving(user)
	msg = logic.NewUserLeaveMessage(user)
	logic.Broadcaster.Broadcast(msg)
	log.Println("user: ", nickname, "leaves chat")

	// 根据读取时的错误执行不同的Close
	if err == nil {
		conn.Close(websocket.StatusNormalClosure, "")
	} else {
		log.Println("read from client error:", err)
		conn.Close(websocket.StatusInternalError, "Read from client error")
	}
}
