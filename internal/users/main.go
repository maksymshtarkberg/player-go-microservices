package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/maksymshtarkberg/music-player-go/internal/database"
	"github.com/maksymshtarkberg/music-player-go/pkg/models"
	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	nc *nats.Conn
)

func main() {
	var err error

	database.InitMongo("mongodb://root:example@localhost:27017", "musicDB")

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	nc.Subscribe("users.register", handleRegister)
	nc.Subscribe("users.get", handleGetUser)

	log.Println("Server users is running...")

	select {}
}

func HashPassword(password string) string {
	hash := sha256.New()
	hash.Write([]byte(password))
	return hex.EncodeToString(hash.Sum(nil))
}

func handleRegister(m *nats.Msg) {
	log.Print("subscribe succsessfull")
	var user models.User
	err := json.Unmarshal(m.Data, &user)
	if err != nil {
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Invalid data: %v"}`, err)))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := database.GetCollection("users")

	user.Password = HashPassword(user.Password)

	_, err = collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			nc.Publish(m.Reply, []byte(`{"error": "User already exists"}`))
			return
		}
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Failed to register user: %v"}`, err)))
		return
	}

	responseData, _ := json.Marshal(map[string]string{
		"status": "User registered successfully",
		"userID": user.ID,
	})

	nc.Publish(m.Reply, responseData)
}

func handleGetUser(m *nats.Msg) {
	username := string(m.Data)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := database.GetCollection("users")

	var user models.User
	err := collection.FindOne(ctx, map[string]interface{}{"username": username}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			nc.Publish(m.Reply, []byte("false"))
			return
		}
		nc.Publish(m.Reply, []byte(fmt.Sprintf(`{"error": "Failed to retrieve user: %v"}`, err)))
		return
	}

	responseData, _ := json.Marshal(user)
	nc.Publish(m.Reply, responseData)
}
