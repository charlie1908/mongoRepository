package mongo

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type Tile struct {
	ID      int  `bson:"ID"`
	Number  int  `bson:"Number"`
	Color   int  `bson:"Color"`
	IsJoker bool `bson:"IsJoker"`
	IsOkey  bool `bson:"IsOkey"`
	IsOpend bool `bson:"IsOpend"`
	GroupID *int `bson:"GroupID,omitempty"` // sadece açık taşlar için atanır
	OrderID *int `bson:"OrderID,omitempty"` // UI'dan gelen sıralama
	X       *int `bson:"X,omitempty"`       // UI sıralaması için (isteğe bağlı)
	Y       *int `bson:"Y,omitempty"`       // UI grubu için (isteğe bağlı)
}

type LogEntry struct {
	ID        primitive.ObjectID `bson:"_id"`
	LogID     string             `bson:"LogID"`     // Benzersiz log ID'si (örnek: UUID)
	DateTime  time.Time          `bson:"DateTime"`  // İnsan okunabilir zaman
	TimeStamp time.Time          `bson:"TimeStamp"` // Query ve sıralama için kullanılan zaman

	OrderID    int64  `bson:"OrderID"`
	UserID     int64  `bson:"UserID"`
	UserName   string `bson:"UserName"`
	ActionType int    `bson:"ActionType"`
	ActionName string `bson:"ActionName"`
	Message    string `bson:"Message"`
	ModuleName string `bson:"ModuleName"`

	GameID    string `bson:"GameID"`
	RoomID    string `bson:"RoomID"`
	MatchID   string `bson:"MatchID,omitempty"`   // Opsiyonel eşleşme ID
	SessionID string `bson:"SessionID,omitempty"` // Opsiyonel kullanıcı oturumu

	Tiles []Tile `bson:"Tiles,omitempty"`

	PenaltyReasonID   *int     `bson:"PenaltyReasonID,omitempty"`
	PenaltyReason     *string  `bson:"PenaltyReason,omitempty"`
	PenaltyMultiplier *float64 `bson:"PenaltyMultiplier,omitempty"`
	PenaltyPoints     *int     `bson:"PenaltyPoints,omitempty"`

	ScoreBefore *int `bson:"ScoreBefore,omitempty"`
	ScoreAfter  *int `bson:"ScoreAfter,omitempty"`
	ScoreDelta  *int `bson:"ScoreDelta,omitempty"`

	HadOkeyTile            *bool `bson:"HadOkeyTile,omitempty"`
	OpenedFivePairsButLost *bool `bson:"OpenedFivePairsButLost,omitempty"`
	OkeyUsedInFinish       *bool `bson:"OkeyUsedInFinish,omitempty"`

	ReconnectDelaySeconds     *float64 `bson:"ReconnectDelaySeconds,omitempty"`
	GameDurationSeconds       *float64 `bson:"GameDurationSeconds,omitempty"`
	PlayerReactionTimeSeconds *float64 `bson:"PlayerReactionTimeSeconds,omitempty"`

	IPAddress string `bson:"IPAddress"`
	Browser   string `bson:"Browser"`
	Device    string `bson:"Device"`
	Platform  string `bson:"Platform"`

	ErrorCode *int                   `bson:"ErrorCode,omitempty"`
	ExtraData map[string]interface{} `bson:"ExtraData,omitempty"`
}

type User struct {
	ID       int64  `bson:"_id"`
	UserName string `bson:"UserName"`
	Email    string `bson:"Email"`
}
