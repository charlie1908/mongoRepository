package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"mongoDBRepository/mongo"
	_ "mongoDBRepository/mongo"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mongo.InitMongoClient("mongodb://localhost:27017")
	okeyColl := client.Database("101Okey").Collection("Log_Entry")

	okeyRepo := mongo.MongoRepository[mongo.LogEntry]{Collection: okeyColl}

	//EXAMPLE-1
	//Update Record-------------
	id := mongo.ToObjectID("6851ba6fc44f8526c6643045")

	// Filter: _id ile sorgula
	filter := mongo.NewQueryBuilder().
		Where("_id", mongo.OpTypes.Eq, id).
		Build()

	// 🔍 1. FindOne ile getir
	entry, err := okeyRepo.FindOne(ctx, filter)
	if err != nil {
		panic(fmt.Sprintf("Belge bulunamadı: %v", err))
	}

	fmt.Println("Güncellenecek kayıt:", entry.LogID)

	// 🔄 2. UpdateOne ile güncelle
	update := map[string]interface{}{
		"ActionName": "UpdatedAction",
		"Message":    "Bu mesaj güncellendi.",
		"DateTime":   time.Now(),
	}

	err = okeyRepo.UpdateOne(ctx, filter, update, false) // upsert=false
	if err != nil {
		panic(fmt.Sprintf("Güncelleme başarısız: %v", err))
	}

	fmt.Println("Başarıyla güncellendi.")
	//Updated Record-------------

	//EXAMPLE-2
	// QueryBuilder örneği
	filter2 := mongo.NewQueryBuilder().
		Where("_id", mongo.OpTypes.Eq, mongo.ToObjectID("6851ba6fc44f8526c6643045")).
		//Where("DateTime", mongo.OpTypes.Gte, mongo.ToTimeRFC3339("2025-06-17T18:50:14.869Z")).
		And("HadOkeyTile", mongo.OpTypes.Gte, true).
		Build()

	okeyLog, err := okeyRepo.Find(ctx, filter2, nil, nil)
	if err != nil {
		panic(err)
	}

	for _, o := range okeyLog {
		fmt.Printf("UserName: %s - MatchID: %s - Date: %s, Message: %s\n", o.UserName, o.MatchID, o.Message, o.DateTime)
	}

	//EXAMPLE-3
	//Aggregation
	builder := mongo.NewAggregateBuilder().
		Match(bson.M{"ActionType": 6}).
		Group("$UserID", bson.M{"total": bson.M{"$sum": 1}}).
		Sort("total", -1).
		Project(bson.M{
			"userId": "$_id",
			"total":  1,
			"_id":    0,
		}).
		Limit(10)

	results, err := okeyRepo.Aggregate(ctx, builder)
	for _, doc := range results {
		fmt.Printf("UserID: %v - Toplam Log: %v\n", doc["userId"], doc["total"])
	}
	//------------------

	//EXAMPLE-4
	//Normal Desc Select By DateTime
	// 1. Filtre: tüm kayıtlar
	filterSelect := mongo.NewQueryBuilder().Build()

	sort := &mongo.SortOption{
		Field:     "DateTime",
		Ascending: false, // DESC
	}

	direction := 1
	if !sort.Ascending {
		direction = -1
	}

	page := 2
	pageSize := int64(2)
	pagination := &mongo.Pagination{
		Limit: pageSize,
		Skip:  (int64(page) - 1) * pageSize,
	}

	// 3. Projeksiyon: sadece gerekli alanlar
	projection := bson.M{
		"Message":  1,
		"UserName": 1,
		"UserID":   1,
		"OrderID":  1,
		"DateTime": 1,
		"_id":      0,
	}

	// 4. Find doğrudan Collection üstünden (repo.Find ile değil)
	findOptions := options.Find().
		SetSort(bson.D{{Key: sort.Field, Value: direction}}).
		SetProjection(projection).
		SetLimit(3). // Top 3
		SetSkip(pagination.Skip)

	cursor, err := okeyRepo.Collection.Find(ctx, filterSelect, findOptions)
	if err != nil {
		log.Fatal("Mongo Find hatası:", err)
	}
	defer cursor.Close(ctx)

	// 5. Sonuçları yazdır
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			log.Fatal(err)
		}

		message := doc["Message"]
		userName := doc["UserName"]
		userID := doc["UserID"]
		orderID := doc["OrderID"]
		dateTime := doc["DateTime"].(primitive.DateTime)

		fmt.Printf(
			"[🕒 %s] 🧑 %v (ID: %v) - 📦 OrderID: %v - 💬 %v\n",
			dateTime.Time().Format("2006-01-02 15:04:05"),
			userName, userID, orderID, message,
		)
		//-------------------
	}
	//EXAMPLE-5
	//AND OR QUERY EXAMPLE
	/*	SELECT * FROM logs
		WHERE (UserID = 123 AND ActionType = 2)
		OR (UserID = 456 AND ActionType = 3)*/

	// OR bloğu: iki ayrı AND kombinasyonu içeriyor
	orFilter := mongo.OrQuery(
		mongo.AndQuery(
			mongo.NewQueryBuilder().
				Where("UserID", mongo.OpTypes.Eq, 42).
				And("_id", mongo.OpTypes.Eq, mongo.ToObjectID("6851ba6fc44f8526c6643045")).
				Build(),
		),
		mongo.AndQuery(
			mongo.NewQueryBuilder().
				Where("UserID", mongo.OpTypes.Eq, 42).
				And("_id", mongo.OpTypes.Eq, mongo.ToObjectID("6851b616f7a2b7812087e845")).
				Build(),
		),
	)
	orResults, err := okeyRepo.Find(ctx, orFilter, nil, nil)
	if err != nil {
		log.Fatal("Sorgu hatası:", err)
	}

	for _, log := range orResults {
		fmt.Printf("UserID: %v - Action: %v\n", log.UserID, log.ActionType)
	}
	//AND OR QUERY EXAMPLE FINISHED------------
	//EXAMPLE-6
	//Lookup() [JOIN]
	builderLookup := mongo.NewAggregateBuilder().
		Lookup(
			"Users",  // from: diğer koleksiyon
			"UserID", // localField: LogEntry.UserID
			"_id",    // foreignField: User._id
			"user",   // as: sonuç alanı
		).
		Unwind("user"). // user array olduğu için patlatıyoruz
		Project(bson.M{
			"UserID":   1,
			"UserName": "$user.UserName",
			"Email":    "$user.Email",
			"Message":  1,
			"DateTime": 1,
			"_id":      0,
		}).
		Sort("DateTime", -1).
		Limit(3)

	resultLookup, err := okeyRepo.Aggregate(ctx, builderLookup)
	if err != nil {
		log.Fatal("Aggregate hatası:", err)
	}

	for _, doc := range resultLookup {
		fmt.Printf("[🕒 %v] 👤 %v <%v> - 💬 %v\n",
			doc["DateTime"].(primitive.DateTime).Time().Format("2006-01-02 15:04:05"),
			doc["UserName"],
			doc["Email"],
			doc["Message"],
		)
	}
	//EXAMPLE - 7
	//Total Count By Email
	builderGroupbyEmail := mongo.NewAggregateBuilder().
		Lookup(
			"Users",  // from
			"UserID", // localField
			"_id",    // foreignField
			"user",   // as
		).
		Unwind("user").
		Group("$user.Email", bson.M{
			"TotalLogs": bson.M{"$sum": 1},
		}).
		Sort("TotalLogs", -1).
		Limit(10)

	resultGroup, err := okeyRepo.Aggregate(ctx, builderGroupbyEmail)
	if err != nil {
		log.Fatal("Aggregate error:", err)
	}

	for _, doc := range resultGroup {
		email := doc["_id"]
		count := doc["TotalLogs"]
		fmt.Printf("📧 %v => 📊 %v log\n", email, count)
	}
}
