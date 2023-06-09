package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type User struct {
	ID             int
	Addr           string
	EnterAt        time.Time
	MessageChannel chan string
}

// Message 给用户发送的消息
type Message struct {
	OwnerID int
	Content string
}

var (
	// 新用户到来, 通过该channel进行登记
	enteringChannel = make(chan *User)
	// 用户离开, 通过该channel进行登记
	leavingChannel = make(chan *User)
	// 广播专用的用户普通消息channel, 缓冲是尽可能避免出现异常情况堵塞
	messageChannel = make(chan Message, 8)
)

func (u *User) String() string {
	return u.Addr + ", UID:" + strconv.Itoa(u.ID) + ", Enter At:" +
		u.EnterAt.Format("2006-01-02 15:04:05+8000")
}

func main() {
	listener, err := net.Listen("tcp", ":2020")
	if err != nil {
		panic(err)
	}

	go broadcaster()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Panicln(err)
			continue
		}
		go handleConn(conn)
	}
}

// broadcaster 用于记录聊天室用户, 并进行消息广播
// 1. 新用户进来 2.用户普通消息 3.用户离开
func broadcaster() {
	users := make(map[*User]struct{})

	for {
		select {
		case user := <-enteringChannel:
			// 新用户进入
			users[user] = struct{}{}
		case user := <-leavingChannel:
			// 用户离开
			delete(users, user)
			// 避免goroutine泄露
			close(user.MessageChannel)
		case msg := <-messageChannel:
			// 给所有在线的用户发送消息
			for user := range users {
				if user.ID == msg.OwnerID {
					continue
				}
				user.MessageChannel <- msg.Content
			}
		}
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()

	// 1. 新用户进来, 构建该用户的实例
	user := &User{
		ID:             GenUserID(),
		Addr:           conn.RemoteAddr().String(),
		EnterAt:        time.Now(),
		MessageChannel: make(chan string, 8),
	}

	// 2. 当前在一个新的goroutine中，用来进行读操作, 因此需要开一个goroutine用于写操作
	// 读写goroutine之间可以通过channel通信
	go sendMessage(conn, user.MessageChannel)

	// 3. 给当前用户发送欢迎消息, 告知所有用户有新用户的到来
	user.MessageChannel <- "Welcome, " + user.String()
	msg := Message{
		OwnerID: user.ID,
		Content: "user:`" + strconv.Itoa(user.ID) + "` has enter",
	}
	messageChannel <- msg

	// 4. 将该记录到全局的用户列表中, 避免用锁
	enteringChannel <- user

	//控制超时用户踢出
	var userActive = make(chan struct{})
	go func() {
		d := 1 * time.Minute
		timer := time.NewTimer(d)
		for {
			select {
			case <-timer.C:
				conn.Close()
			case <-userActive:
				timer.Reset(d)
			}
		}
	}()

	// 5. 循环读取用户的输入
	input := bufio.NewScanner(conn)
	for input.Scan() {
		msg.Content = strconv.Itoa(user.ID) + ":" + input.Text()
		messageChannel <- msg

		//用户活跃
		userActive <- struct{}{}
	}
	if err := input.Err(); err != nil {
		log.Panicln("读取错误: ", err)
	}

	// 6. 用户离开
	leavingChannel <- user
	msg.Content = "user:`" + strconv.Itoa(user.ID) + "` has left"
	messageChannel <- msg
}

func sendMessage(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintln(conn, msg)
	}
}

var (
	globalID int
	idLocker sync.Mutex
)

func GenUserID() int {
	idLocker.Lock()
	defer idLocker.Unlock()

	globalID++
	return globalID
}
