package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	pb "github.com/maksymshtarkberg/music-player-go/internal/songs/proto"
	"github.com/nats-io/nats.go"
)

type Server struct {
	pb.UnimplementedSongServiceServer
	mongoClient *mongo.Client
	natsConn    *nats.Conn
}

func (s *Server) UploadSong(ctx context.Context, req *pb.UploadSongRequest) (*pb.UploadSongResponse, error) {
	log.Print(req.SongFile)
	return &pb.UploadSongResponse{
		Message: "Song uploaded successfully",
		Status:  "success",
	}, nil
}

func main() {
	clientOptions := options.Client().ApplyURI("mongodb://root:example@mongo_db:27017")
	mongoClient, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.TODO())

	natsConn, err := nats.Connect("nats://nats_messages:4222")
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsConn.Close()

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024),
		grpc.MaxSendMsgSize(10*1024*1024),
	)
	pb.RegisterSongServiceServer(grpcServer, &Server{
		mongoClient: mongoClient,
		natsConn:    natsConn,
	})

	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen on port 8080: %v", err)
	}

	log.Println("gRPC server running on port 8080")
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

// func (s *Server) UploadSong(ctx context.Context, req *pb.UploadSongRequest) (*pb.UploadSongResponse, error) {
// 	db := s.mongoClient.Database("musicDB")
// 	bucket, err := gridfs.NewBucket(db)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open GridFS bucket: %v", err)
// 	}

// 	songFileID := primitive.NewObjectID()
// 	songUploadStream, err := bucket.OpenUploadStreamWithID(songFileID, string(req.SongFile))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open song upload stream: %v", err)
// 	}
// 	defer songUploadStream.Close()

// 	if _, err := songUploadStream.Write(req.SongFile); err != nil {
// 		return nil, fmt.Errorf("failed to upload song file: %v", err)
// 	}

// 	// Upload album cover to GridFS.
// 	albumCoverID := primitive.NewObjectID()
// 	albumCoverUploadStream, err := bucket.OpenUploadStreamWithID(albumCoverID, req.Album)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open album cover upload stream: %v", err)
// 	}
// 	defer albumCoverUploadStream.Close()

// 	if _, err := albumCoverUploadStream.Write(req.AlbumCover); err != nil {
// 		return nil, fmt.Errorf("failed to upload album cover: %v", err)
// 	}

// 	songDoc := bson.M{
// 		"title":        req.Title,
// 		"artist":       req.Artist,
// 		"album":        req.Album,
// 		"description":  req.Description,
// 		"uploadedBy":   req.UploadedBy,
// 		"songFileID":   songFileID,
// 		"albumCoverID": albumCoverID,
// 	}

// 	collection := db.Collection("songs")
// 	_, err = collection.InsertOne(context.TODO(), songDoc)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to save song metadata: %v", err)
// 	}

// 	err = s.natsConn.Publish("song.uploaded", []byte(fmt.Sprintf("Song %s by %s uploaded", req.Title, req.Artist)))
// 	if err != nil {
// 		log.Printf("Failed to publish to NATS: %v", err)
// 	}

// 	return &pb.UploadSongResponse{
// 		Message: "Song uploaded successfully",
// 		Status:  "success",
// 		SongId:  songFileID.Hex(),
// 	}, nil
// }

// func (s *Server) StreamSongFile(req *pb.StreamSongFileRequest, stream pb.SongService_StreamSongFileServer) error {
// 	db := s.mongoClient.Database("music_db")
// 	bucket, err := gridfs.NewBucket(db)
// 	if err != nil {
// 		return fmt.Errorf("failed to open GridFS bucket: %v", err)
// 	}

// 	objectID, err := primitive.ObjectIDFromHex(req.SongFileId)
// 	if err != nil {
// 		return fmt.Errorf("invalid song file ID: %v", err)
// 	}

// 	downloadStream, err := bucket.OpenDownloadStream(objectID)
// 	if err != nil {
// 		return fmt.Errorf("failed to open song download stream: %v", err)
// 	}
// 	defer downloadStream.Close()

// 	buffer := make([]byte, 4096)
// 	for {
// 		n, err := downloadStream.Read(buffer)
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			return fmt.Errorf("failed to read song file: %v", err)
// 		}

// 		err = stream.Send(&pb.StreamSongFileResponse{Chunk: buffer[:n]})
// 		if err != nil {
// 			return fmt.Errorf("failed to send song chunk: %v", err)
// 		}
// 	}

// 	return nil
// }
