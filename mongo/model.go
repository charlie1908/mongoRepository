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
	IsDeleted bool                   `bson:"IsDeleted,omitempty"`
	DeletedBy string                 `bson:"DeletedBy,omitempty"`
	DeletedAt *time.Time             `bson:"DeletedAt,omitempty"`
}

type User struct {
	ID        int64              `bson:"_id,omitempty"`
	UserName  string             `bson:"UserName,omitempty"`
	Email     string             `bson:"Email,omitempty"`
	OrderID   primitive.ObjectID `bson:"OrderID,omitempty"`
	IsDeleted bool               `bson:"IsDeleted,omitempty"`
	DeletedBy string             `bson:"DeletedBy,omitempty"`
	DeletedAt *time.Time         `bson:"DeletedAt,omitempty"`
}

// Users._id: INT (örneğin)
// Users.OrderID: ObjectID  ->  Orders._id: ObjectID

type Order struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Code      string             `bson:"Code,omitempty"`
	Total     float64            `bson:"Total,omitempty"`
	IsDeleted bool               `bson:"IsDeleted,omitempty"`
	DeletedBy string             `bson:"DeletedBy,omitempty"`
	DeletedAt *time.Time         `bson:"DeletedAt,omitempty"`
}

// Köprü: her satır bir siparişi işaret eder
// UserID: int32  — Orders._id: ObjectID
type UserOrders struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    int32              `bson:"UserID"`
	OrderID   primitive.ObjectID `bson:"OrderID"`
	IsDeleted bool               `bson:"IsDeleted,omitempty"`
	DeletedBy string             `bson:"DeletedBy,omitempty"`
	DeletedAt *time.Time         `bson:"DeletedAt,omitempty"`
}
