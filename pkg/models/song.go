package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Song struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Title          string             `bson:"title"`
	Artist         string             `bson:"artist"`
	Album          string             `bson:"album"`
	Description    string             `bson:"description"`
	UploadedBy     string             `bson:"uploadedBy"`
	SongFileID     primitive.ObjectID `bson:"songFileID"`
	AlbumCoverID   primitive.ObjectID `bson:"albumCoverID"`
	SongFileData   []byte             `bson:"-"`
	AlbumCoverData []byte             `bson:"-"`
	UploadedAt     primitive.DateTime `bson:"uploadedAt"`
}
