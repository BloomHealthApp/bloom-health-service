package dao

import "time"

// DMThread is the one conversation between two friends. Its ID is their PairKey, so there is at most one per pair.
// NextSeq numbers the messages; Last is the newest message, for the inbox.
type DMThread struct {
	ID      string               `bson:"_id"`
	Members []string             `bson:"members"`
	NextSeq int64                `bson:"nextSeq"`
	Last    *DMLast              `bson:"last,omitempty"`
	ReadAt  map[string]time.Time `bson:"readAt,omitempty"`
}

// DMLast is a copy of a thread's newest message.
type DMLast struct {
	Seq       int64     `bson:"seq"`
	SenderID  string    `bson:"senderId"`
	Text      string    `bson:"text"`
	CreatedAt time.Time `bson:"createdAt"`
}

// DirectMessage is one text message in a DMThread. ID is chosen by the sending app, so a retried send is recognised.
type DirectMessage struct {
	ID        string    `bson:"_id"`
	PairKey   string    `bson:"pairKey"`
	Seq       int64     `bson:"seq"`
	SenderID  string    `bson:"senderId"`
	Text      string    `bson:"text"`
	CreatedAt time.Time `bson:"createdAt"`
}

// PushToken is one device's Expo push token. A user can have several devices.
type PushToken struct {
	Token     string    `bson:"_id"`
	UserID    string    `bson:"userId"`
	Platform  string    `bson:"platform"`
	UpdatedAt time.Time `bson:"updatedAt"`
}
