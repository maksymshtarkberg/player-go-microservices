package main

import (
	"context"
	"log"

	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	nc *nats.Conn
)

func main() {
	var err error

	clientOptions := options.Client().ApplyURI("mongodb://root:example@localhost:27017")
	mongoClient, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer mongoClient.Disconnect(context.TODO())
	db := mongoClient.Database("musicDB")

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	nc.Subscribe("songs.upload", HandleUploadSong(nc, db))
	nc.Subscribe("songs.user", HandleGetUserSongs(nc, db))
	nc.Subscribe("songs.all", HandleGetAllSongs(nc, db))
	nc.Subscribe("songs.update", HandleUpdateSongMetadata(nc, db))
	nc.Subscribe("songs.delete", HandleDeleteSong(nc, db))

	log.Println("Server songs is running...")

	select {}
}
