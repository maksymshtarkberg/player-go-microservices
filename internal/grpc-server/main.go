package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"

	pb "github.com/maksymshtarkberg/music-player-go/internal/grpc-server/proto"
	"github.com/nats-io/nats.go"
)

type Server struct {
	pb.UnimplementedSongServiceServer
	mongoClient *mongo.Client
	natsConn    *nats.Conn
}

func (s *Server) UploadSong(ctx context.Context, req *pb.UploadSongRequest) (*pb.UploadSongResponse, error) {
	db := s.mongoClient.Database("musicDB")
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		return nil, fmt.Errorf("failed to open GridFS bucket: %v", err)
	}

	songFileID := primitive.NewObjectID()
	songUploadStream, err := bucket.OpenUploadStreamWithID(songFileID, string(req.SongFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open song upload stream: %v", err)
	}
	defer songUploadStream.Close()

	if _, err := songUploadStream.Write(req.SongFile); err != nil {
		return nil, fmt.Errorf("failed to upload song file: %v", err)
	}

	albumCoverID := primitive.NewObjectID()
	albumCoverUploadStream, err := bucket.OpenUploadStreamWithID(albumCoverID, req.Album)
	if err != nil {
		return nil, fmt.Errorf("failed to open album cover upload stream: %v", err)
	}
	defer albumCoverUploadStream.Close()

	if _, err := albumCoverUploadStream.Write(req.AlbumCover); err != nil {
		return nil, fmt.Errorf("failed to upload album cover: %v", err)
	}

	songMetadata := &pb.SongMetadata{
		Title:        req.Title,
		Artist:       req.Artist,
		Album:        req.Album,
		Description:  req.Description,
		UploadedBy:   req.UploadedBy,
		SongFileID:   songFileID.Hex(),
		AlbumCoverID: albumCoverID.Hex(),
	}
	metadataData, err := proto.Marshal(songMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal song metadata: %v", err)
	}

	err = s.natsConn.Publish("songs.upload", metadataData)
	if err != nil {
		return nil, fmt.Errorf("failed to publish to NATS: %v", err)
	}

	return &pb.UploadSongResponse{
		Message: "Song uploaded successfully",
		Status:  "success",
		SongId:  songFileID.Hex(),
		CoverId: albumCoverID.Hex(),
	}, nil
}

func (s *Server) StreamSongFile(req *pb.StreamSongFileRequest, stream pb.SongService_StreamSongFileServer) error {
	db := s.mongoClient.Database("musicDB")
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		return fmt.Errorf("failed to open GridFS bucket: %v", err)
	}
	log.Printf("Attempting to stream song file with ID: %s", req.SongFileId)

	objectID, err := primitive.ObjectIDFromHex(req.SongFileId)
	if err != nil {
		return fmt.Errorf("invalid song file ID: %v", err)
	}

	downloadStream, err := bucket.OpenDownloadStream(objectID)
	if err != nil {
		return fmt.Errorf("failed to open song download stream: %v", err)
	}
	defer downloadStream.Close()

	buffer := make([]byte, 4096)
	for {
		n, err := downloadStream.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read song file: %v", err)
		}

		err = stream.Send(&pb.StreamSongFileResponse{Chunk: buffer[:n]})
		if err != nil {
			return fmt.Errorf("failed to send song chunk: %v", err)
		}
	}

	return nil
}

func (s *Server) StreamAlbumCover(req *pb.StreamAlbumCoverRequest, stream pb.SongService_StreamAlbumCoverServer) error {
	db := s.mongoClient.Database("musicDB")
	bucket, err := gridfs.NewBucket(db)
	if err != nil {
		return fmt.Errorf("failed to open GridFS bucket: %v", err)
	}
	log.Printf("Attempting to stream album cover with ID: %s", req.AlbumCoverId)

	objectID, err := primitive.ObjectIDFromHex(req.AlbumCoverId)
	if err != nil {
		return fmt.Errorf("invalid album cover ID: %v", err)
	}

	downloadStream, err := bucket.OpenDownloadStream(objectID)
	if err != nil {
		return fmt.Errorf("failed to open album cover download stream: %v", err)
	}
	defer downloadStream.Close()

	buffer := make([]byte, 4096)
	for {
		n, err := downloadStream.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read album cover file: %v", err)
		}

		err = stream.Send(&pb.StreamAlbumCoverResponse{Chunk: buffer[:n]})
		if err != nil {
			return fmt.Errorf("failed to send album cover chunk: %v", err)
		}
	}

	return nil
}

func (s *Server) GetUserSongs(ctx context.Context, req *pb.GetUserSongsRequest) (*pb.GetUserSongsResponse, error) {
	userId := req.GetUserId()

	natsReq := []byte(userId)
	log.Printf("Attempting to get user songs with user ID: %s", userId)

	msg, err := s.natsConn.Request("songs.user", natsReq, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get songs from NATS: %v", err)
	}

	var response pb.GetUserSongsResponse
	err = proto.Unmarshal(msg.Data, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

func (s *Server) GetAllSongs(ctx context.Context, req *pb.GetAllSongsRequest) (*pb.GetAllSongsResponse, error) {
	natsReq := []byte{}

	log.Println("Attempting to get all songs via NATS")

	msg, err := s.natsConn.Request("songs.all", natsReq, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get all songs from NATS: %v", err)
	}

	var response pb.GetAllSongsResponse
	err = proto.Unmarshal(msg.Data, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

func (s *Server) UpdateSongMetadata(ctx context.Context, req *pb.UpdateSongMetadataRequest) (*pb.UpdateSongMetadataResponse, error) {
	natsReq := &pb.UpdateSongMetadataRequest{
		SongId:      req.GetSongId(),
		Title:       req.GetTitle(),
		Artist:      req.GetArtist(),
		Album:       req.GetAlbum(),
		Description: req.GetDescription(),
	}

	requestData, err := proto.Marshal(natsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	msg, err := s.natsConn.Request("songs.update", requestData, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to request NATS: %v", err)
	}

	var response pb.UpdateSongMetadataResponse
	err = proto.Unmarshal(msg.Data, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

func (s *Server) DeleteSong(ctx context.Context, req *pb.DeleteSongRequest) (*pb.DeleteSongResponse, error) {
	songFileId := req.GetSongFileId()
	albumCoverId := req.GetAlbumCoverId()
	songId := req.GetSongId()

	natsReq := []byte(songId)

	msg, err := s.natsConn.Request("songs.delete", natsReq, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to request NATS: %v", err)
	}

	var response pb.DeleteSongResponse
	err = proto.Unmarshal(msg.Data, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if response.GetSuccess() {
		db := s.mongoClient.Database("musicDB")
		bucket, err := gridfs.NewBucket(db)
		if err != nil {
			return nil, fmt.Errorf("failed to open GridFS bucket: %v", err)
		}

		objectSongFileID, err := primitive.ObjectIDFromHex(songFileId)
		if err != nil {
			return nil, fmt.Errorf("invalid song file ID: %v", err)
		}
		if err := bucket.Delete(objectSongFileID); err != nil {
			return nil, fmt.Errorf("failed to delete song file: %v", err)
		}

		objectAlbumCoverID, err := primitive.ObjectIDFromHex(albumCoverId)
		if err != nil {
			return nil, fmt.Errorf("invalid album cover ID: %v", err)
		}
		if err := bucket.Delete(objectAlbumCoverID); err != nil {
			return nil, fmt.Errorf("failed to delete album cover: %v", err)
		}
	}

	return &response, nil
}

func main() {
	clientOptions := options.Client().ApplyURI("mongodb://root:example@mongo_db:27017")
	mongoClient, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.TODO())

	natsConn, err := nats.Connect("nats://nats_msg:4222")
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
