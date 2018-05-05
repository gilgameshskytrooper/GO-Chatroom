package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type User struct {
	ID   string
	conn *websocket.Conn
}
type Store struct {
	Users []*User
	sync.Mutex
}

type Message struct {
	DeliveryID string `json:"toid"`
	SenderID   string `json:"senderid"`
	Content    string `json:"content"`
}

var (
	gStore      *Store
	gPubSubConn *redis.PubSubConn
	gRedisConn  = func() (redis.Conn, error) {
		return redis.Dial("tcp", ":6379")
	}
	redisconn redis.Conn
)

func init() {
	gStore = &Store{
		Users: make([]*User, 0, 1),
	}
	redisconn, _ = redis.Dial("tcp", ":6379")
	// redisconn, _ := redis.DialURL(os.Getenv("REDISLOCATION"))
}

func (s *Store) newUser(username string, conn *websocket.Conn) *User {
	u := &User{
		ID:   username,
		conn: conn,
	}
	if err := gPubSubConn.Subscribe(u.ID); err != nil {
		panic(err)
	}
	s.Lock()
	defer s.Unlock()
	s.Users = append(s.Users, u)
	redisconn.Do("SADD", "users", username)
	return u
}

func (s *Store) deleteUser(username string) {
	s.Lock()
	defer s.Unlock()
	for index, elem := range s.Users {
		if elem.ID == username {
			s.Users = append(s.Users[0:index], s.Users[index+1:]...)
		}
	}
	redisconn.Do("SREM", "users", username)
}

func deliverMessages() {
	for {
		switch v := gPubSubConn.Receive().(type) {
		case redis.Message:
			gStore.findAndDeliver(v.Channel, string(v.Data))

		case redis.Subscription:
			log.Printf("subscription message: %s: %s %d\n", v.Channel, v.Kind, v.Count)

		case error:
			log.Println("error pub/sub, delivery has stopped")
			return
		}
	}
}

func split_content(content string) (string, string) {
	for index, elem := range content {
		if string(elem) == "]" {
			return content[1:index], content[index+1:]

		}
	}
	return "", ""
}

func (s *Store) findAndDeliver(userID string, fullcontent string) {
	senderid, content := split_content(fullcontent)
	m := Message{
		Content:  content,
		SenderID: senderid,
	}
	for _, u := range s.Users {
		if u.ID == userID {
			if err := u.conn.WriteJSON(m); err != nil {
				log.Printf("error on message delivery e: %s\n", err)
			} else {
				log.Printf("user %s found, message sent\n", userID)
			}
			return
		}
	}
	log.Printf("user %s not found at our store\n", userID)
}

func main() {
	gRedisConn, err := gRedisConn()
	if err != nil {
		panic(err)
	}
	defer gRedisConn.Close()
	gPubSubConn = &redis.PubSubConn{Conn: gRedisConn}
	defer gPubSubConn.Close()
	go deliverMessages()
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "index.html") })
	r.HandleFunc("/ws/{username}", gStore.wsHandler)
	log.Printf("server started at %s\n", ":8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Store) wsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrader error %s\n" + err.Error())
		return
	}
	u := gStore.newUser(username, conn)
	log.Printf("user %s joined\n", u.ID)

	for {
		var m Message
		if err := u.conn.ReadJSON(&m); err != nil {
			log.Printf("User closed connection")
			s.deleteUser(username)
			return
		}
		if c, err := gRedisConn(); err != nil {
			log.Printf("error on redis conn. %s\n", err)
		} else {
			if m.DeliveryID == "" {
				users, _ := redis.Strings(redisconn.Do("SMEMBERS", "users"))
				for _, elem := range users {
					c.Do("PUBLISH", elem, "["+string(m.SenderID)+"]"+string(m.Content))
				}
			} else {
				c.Do("PUBLISH", m.DeliveryID, "["+string(m.SenderID)+"]"+string(m.Content))
			}
		}
	}
}
