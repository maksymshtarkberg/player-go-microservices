package main

import (
	"context"
	"fmt"
	"time"

	"log"

	"github.com/nats-io/nats.go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/mongo"

	pb "github.com/maksymshtarkberg/music-player-go/internal/grpc-server/proto"
	"google.golang.org/protobuf/proto"
)

func HandleUploadSong(nc *nats.Conn, db *mongo.Database) func(m *nats.Msg) {
	return func(m *nats.Msg) {
		var metadata pb.SongMetadata
		err := proto.Unmarshal(m.Data, &metadata)
		if err != nil {
			log.Printf("Failed to unmarshal song metadata: %v", err)
			return
		}

		songDoc := bson.M{
			"title":        metadata.Title,
			"artist":       metadata.Artist,
			"album":        metadata.Album,
			"description":  metadata.Description,
			"uploadedBy":   metadata.UploadedBy,
			"songFileID":   metadata.SongFileID,
			"albumCoverID": metadata.AlbumCoverID,
		}

		collection := db.Collection("songs")
		_, err = collection.InsertOne(context.TODO(), songDoc)
		if err != nil {
			log.Printf("Failed to save song metadata: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}

		log.Printf("Song metadata for %s by %s saved successfully", metadata.Title, metadata.Artist)

	}
}

func HandleGetAllSongs(nc *nats.Conn, db *mongo.Database) func(m *nats.Msg) {
	return func(m *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Println("Attempting to send all songs")

		collection := db.Collection("songs")
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			log.Printf("Failed to retrieve songs: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}
		defer cursor.Close(ctx)

		var songs []*pb.SongMetadata
		for cursor.Next(ctx) {
			var songDoc bson.M
			if err := cursor.Decode(&songDoc); err != nil {
				log.Printf("Failed to decode song document: %v", err)
				m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
				return
			}
			songId, ok := songDoc["_id"].(primitive.ObjectID)
			if !ok {
				log.Printf("Failed to cast _id to ObjectID")
				m.Respond([]byte("Error: Failed to cast _id to ObjectID"))
				return
			}

			song := &pb.SongMetadata{
				XId:          songId.Hex(),
				Title:        songDoc["title"].(string),
				Artist:       songDoc["artist"].(string),
				Album:        songDoc["album"].(string),
				Description:  songDoc["description"].(string),
				UploadedBy:   songDoc["uploadedBy"].(string),
				SongFileID:   songDoc["songFileID"].(string),
				AlbumCoverID: songDoc["albumCoverID"].(string),
			}
			songs = append(songs, song)
		}

		if err := cursor.Err(); err != nil {
			log.Printf("Error while iterating songs cursor: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}

		if len(songs) == 0 {
			log.Println("No songs found in the database")
			response := &pb.GetAllSongsResponse{
				Songs: []*pb.SongMetadata{},
			}

			responseData, err := proto.Marshal(response)
			if err != nil {
				log.Printf("Failed to marshal response: %v", err)
				m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
				return
			}

			m.Respond(responseData)
			return
		}

		response := &pb.GetAllSongsResponse{
			Songs: songs,
		}

		responseData, err := proto.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}

		m.Respond(responseData)
	}
}

func HandleGetUserSongs(nc *nats.Conn, db *mongo.Database) func(m *nats.Msg) {
	return func(m *nats.Msg) {
		userId := string(m.Data)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Printf("Attempting to stream songs for user ID: %s", userId)

		collection := db.Collection("songs")
		filter := bson.M{"uploadedBy": userId}
		cursor, err := collection.Find(ctx, filter)
		if err != nil {
			log.Printf("Failed to find songs for user %s: %v", userId, err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}
		defer cursor.Close(ctx)

		var songs []*pb.SongMetadata
		for cursor.Next(ctx) {
			var songDoc bson.M
			if err := cursor.Decode(&songDoc); err != nil {
				log.Printf("Failed to decode song document: %v", err)
				m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
				return
			}

			songId, ok := songDoc["_id"].(primitive.ObjectID)
			if !ok {
				log.Printf("Failed to cast _id to ObjectID")
				m.Respond([]byte("Error: Failed to cast _id to ObjectID"))
				return
			}

			song := &pb.SongMetadata{
				XId:          songId.Hex(),
				Title:        songDoc["title"].(string),
				Artist:       songDoc["artist"].(string),
				Album:        songDoc["album"].(string),
				Description:  songDoc["description"].(string),
				UploadedBy:   songDoc["uploadedBy"].(string),
				SongFileID:   songDoc["songFileID"].(string),
				AlbumCoverID: songDoc["albumCoverID"].(string),
			}
			songs = append(songs, song)
		}

		if err := cursor.Err(); err != nil {
			log.Printf("Error while iterating songs cursor: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}

		if len(songs) == 0 {
			log.Printf("No songs found for user %s", userId)
			response := &pb.GetUserSongsResponse{
				Songs: []*pb.SongMetadata{},
			}

			responseData, err := proto.Marshal(response)
			if err != nil {
				log.Printf("Failed to marshal response: %v", err)
				m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
				return
			}

			m.Respond(responseData)
			return
		}

		response := &pb.GetUserSongsResponse{
			Songs: songs,
		}

		responseData, err := proto.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			m.Respond([]byte(fmt.Sprintf("Error: %v", err)))
			return
		}

		m.Respond(responseData)
	}
}

func HandleUpdateSongMetadata(nc *nats.Conn, db *mongo.Database) func(m *nats.Msg) {
	return func(m *nats.Msg) {
		var req pb.UpdateSongMetadataRequest
		err := proto.Unmarshal(m.Data, &req)
		if err != nil {
			log.Printf("Failed to unmarshal request: %v", err)
			m.Respond([]byte("Error: Failed to unmarshal request"))
			return
		}

		songIdStr := req.GetSongId()
		objectID, err := primitive.ObjectIDFromHex(songIdStr)
		if err != nil {
			log.Printf("Failed to convert song ID to ObjectID: %v", err)
			m.Respond([]byte("Error: Invalid song ID"))
			return
		}

		collection := db.Collection("songs")
		filter := bson.M{"_id": objectID}
		update := bson.M{
			"$set": bson.M{
				"title":       req.GetTitle(),
				"artist":      req.GetArtist(),
				"album":       req.GetAlbum(),
				"description": req.GetDescription(),
			},
		}

		_, err = collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			log.Printf("Failed to update song metadata: %v", err)
			m.Respond([]byte("Error: Failed to update song metadata"))
			return
		}

		response := &pb.UpdateSongMetadataResponse{
			Success: true,
			Message: "Song metadata updated successfully",
		}

		responseData, err := proto.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			m.Respond([]byte("Error: Failed to marshal response"))
			return
		}

		m.Respond(responseData)
	}
}

func HandleDeleteSong(nc *nats.Conn, db *mongo.Database) func(m *nats.Msg) {
	return func(m *nats.Msg) {

		songIdStr := string(m.Data)
		objectID, err := primitive.ObjectIDFromHex(songIdStr)
		if err != nil {
			log.Printf("Failed to convert song ID to ObjectID: %v", err)
			m.Respond([]byte("Error: Invalid song ID"))
			return
		}

		collection := db.Collection("songs")
		filter := bson.M{"_id": objectID}

		result, err := collection.DeleteOne(context.TODO(), filter)
		if err != nil {
			log.Printf("Failed to delete song: %v", err)
			m.Respond([]byte("Error: Failed to delete song"))
			return
		}

		if result.DeletedCount == 0 {
			log.Printf("No song found with ID %s", songIdStr)
			m.Respond([]byte("Error: No song found with the specified ID"))
			return
		}

		response := &pb.DeleteSongResponse{
			Success: true,
			Message: "Song deleted successfully",
		}

		responseData, err := proto.Marshal(response)
		if err != nil {
			log.Printf("Failed to marshal response: %v", err)
			m.Respond([]byte("Error: Failed to marshal response"))
			return
		}

		m.Respond(responseData)
	}
}
