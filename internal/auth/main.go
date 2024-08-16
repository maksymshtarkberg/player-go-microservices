package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/maksymshtarkberg/music-player-go/pkg/models"
	"github.com/nats-io/nats.go"
)

var (
	nc          *nats.Conn
	redisClient *redis.Client
	ctx         = context.Background()
)

func main() {
	var err error

	redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	nc.Subscribe("auth.register", handleRegister)
	nc.Subscribe("auth.authenticate", handleAuthenticate)

	log.Println("Server auth is running...")

	select {}
}

func handleRegister(m *nats.Msg) {
	var user models.User
	err := json.Unmarshal(m.Data, &user)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Invalid data: %v"}`, err)))
		return
	}

	existingToken, err := redisClient.Get(ctx, user.Username).Result()
	if err != nil && err != redis.Nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Redis error: %v"}`, err)))
		return
	}

	if existingToken != "" {
		nc.Publish(m.Reply, []byte(`{"error": "User already registered"}`))
		return
	}

	userExist, err := nc.Request("users.get", []byte(user.Username), 5*nats.DefaultTimeout)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Users service error: %v"}`, err)))
		return
	}
	if string(userExist.Data) != "false" {
		nc.Publish(m.Reply, []byte(`{"error": "User already registered"}`))
		return
	}

	token := fmt.Sprintf("%d", time.Now().UnixNano())
	err = redisClient.Set(ctx, user.Username, token, time.Hour*24*7).Err()
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Redis error: %v"}`, err)))
		return
	}

	response, err := nc.Request("users.register", m.Data, 5*nats.DefaultTimeout)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Users service error: %v"}`, err)))

		return
	}

	nc.Publish(m.Reply, response.Data)
}

func handleAuthenticate(m *nats.Msg) {
	var credentials struct {
		Username string `bson:"username"`
		Password string `bson:"password"`
	}
	err := json.Unmarshal(m.Data, &credentials)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Invalid data: %v"}`, err)))
		return
	}

	response, err := nc.Request("users.get", []byte(credentials.Username), nats.DefaultTimeout)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Users service error: %v"}`, err)))
		return
	}

	if string(response.Data) == "false" {
		nc.Publish(m.Reply, []byte(`{"error": "User not registered"}`))
		return
	}

	var user models.User
	err = json.Unmarshal(response.Data, &user)
	if err != nil {
		nc.Publish(m.Reply, []byte(`{"error": "Invalid username or password"}`))
		return
	}
	hashedPassword := HashPassword(credentials.Password)

	if user.Password != hashedPassword {
		nc.Publish(m.Reply, []byte(`{"error": "Invalid username or password"}`))
		return
	}

	token, err := redisClient.Get(ctx, credentials.Username).Result()
	if err != nil && err != redis.Nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Redis error: %v"}`, err)))
		return
	}

	if token == "" {
		token = fmt.Sprintf("%d", time.Now().UnixNano())
		err = redisClient.Set(ctx, user.Username, token, time.Hour*24*7).Err()
		if err != nil {
			nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Redis error: %v"}`, err)))
			return
		}

		token, err = redisClient.Get(ctx, credentials.Username).Result()
		if err != nil {
			nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Failed to retrieve token: %v"}`, err)))
			return
		}
	}

	responseData, _ := json.Marshal(map[string]string{
		"status": "Authenticated",
		"token":  token,
	})
	nc.Publish(m.Reply, responseData)
}

func HashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return hex.EncodeToString(hash.Sum(nil))
}
