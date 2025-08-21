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
	//var isDeleted = true
	//entry, err := okeyRepo.FindOne(ctx, filter, &isDeleted)
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

	//EXAMPLE-3.1
	//Multi GroupBy [UserID, GameID]
	builder = mongo.NewAggregateBuilder().
		Match(bson.M{"ActionType": 6}).
		Group(
			bson.M{
				"userId": "$UserID", // 👈 birden fazla field
				"gameId": "$GameID",
			},
			bson.M{
				"total": bson.M{"$sum": 1},
			},
		).
		Sort("total", -1).
		Project(bson.M{
			"userId": "$_id.userId",
			"gameId": "$_id.gameId",
			"total":  1,
			"_id":    0,
		}).
		Limit(10)

	results, err = okeyRepo.Aggregate(ctx, builder)
	if err != nil {
		log.Fatal(err)
	}

	for _, doc := range results {
		fmt.Printf("UserID: %v - GameID: %v - Toplam Log: %v\n", doc["userId"], doc["gameId"], doc["total"])
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

	//EXAMPLE - 8 MULTIPLE SORT
	//Normal Desc Select By DateTime
	// 1. Filtre: tüm kayıtlar
	fmt.Printf("MULTIPLE SORT EXAMPLE\n")
	filterSelect = mongo.NewQueryBuilder().Build()

	// Çok alanlı sıralama
	sortOpts := &mongo.SortOptions{
		Fields: []mongo.SortField{
			{Field: "DateTime", Asc: false}, // DESC
			{Field: "_id", Asc: true},       // ASC
		},
	}

	// 3. Projeksiyon: sadece gerekli alanlar
	projection = bson.M{
		"Message":  1,
		"UserName": 1,
		"UserID":   1,
		"OrderID":  1,
		"DateTime": 1,
		"_id":      1,
	}

	// 4. Find doğrudan Collection üstünden (repo.Find ile değil)
	findOptions = options.Find().
		SetSort(mongo.Sorts(sortOpts)).
		SetProjection(projection).
		SetLimit(5)

	cursor, err = okeyRepo.Collection.Find(ctx, filterSelect, findOptions)
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
		id := doc["_id"].(primitive.ObjectID).Hex()

		fmt.Printf(
			"[🕒 %s] 🧑 %v (ID: %v) - 📦 OrderID: %v - 💬 %v - _id: %v\n",
			dateTime.Time().Format("2006-01-02 15:04:05"),
			userName, userID, orderID, message, id,
		)
		//-------------------
	}

	//EXAMPLE-8
	//BULK Update Record - MULTIPLE WHERE-------------
	id = mongo.ToObjectID("6851ba6fc44f8526c6643045")
	//id2 := mongo.ToObjectID("6851b616f7a2b7812087e845")

	// Filter: _id ve _id2 ile sorgula
	filter = mongo.NewQueryBuilder().
		//Where("_id", mongo.OpTypes.In, bson.A{id, id2}).
		Where("_id", mongo.OpTypes.Eq, id).
		Where("UserName", mongo.OpTypes.Eq, "player1").
		Build()

	// 🔍 1. Find ile hepsini getir..
	entries, err := okeyRepo.Find(ctx, filter, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("Belge bulunamadı: %v", err))
	}

	for _, entry := range entries {
		fmt.Println("Güncellenecek kayıt:", entry.ID.Hex())
	}

	// 🔄 2. UpdateOne ile güncelle
	update = map[string]interface{}{
		"Message": "Bu mesaj güncellendi.2",
	}

	updatedCount, err := okeyRepo.BulkUpdate(ctx, filter, update, false) // upsert=false
	if err != nil {
		panic(fmt.Sprintf("Güncelleme başarısız: %v", err))
	}

	fmt.Println(fmt.Sprintf("Tum kayitlar Başarıyla güncellendi. Total Count: [%d]", updatedCount))
	//Updated Record-------------

	//EXAMPLE-9
	//WHEREIN() Example
	id = mongo.ToObjectID("6851ba6fc44f8526c6643045")
	id2 := mongo.ToObjectID("6851b616f7a2b7812087e845")
	filter = mongo.NewQueryBuilder().
		//WhereIn("_id", bson.A{id, id2}).
		WhereIn("_id", id, id2).
		Where("UserName", mongo.OpTypes.Eq, "player1").
		Build()
	entries, err = okeyRepo.Find(ctx, filter, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("Belge bulunamadı: %v", err))
	}
	for _, entry := range entries {
		fmt.Println("WhereIn ile bulunan kayıt:", entry.ID.Hex())
	}

	//EXAMPLE 10
	//BULKINSERT Test

	// User repository oluştur
	userColl := client.Database("101Okey").Collection("Users")
	userRepo := mongo.MongoRepository[mongo.User]{Collection: userColl}

	// Eklenecek kullanıcılar
	users := []mongo.User{
		{
			ID:       4,
			UserName: "player4",
			Email:    "player4@example.com",
		},
		{
			ID:       5,
			UserName: "player5",
			Email:    "player5@example.com",
		},
	}

	// BulkInsert çağır
	if err := userRepo.BulkInsert(ctx, users); err != nil {
		panic(fmt.Sprintf("Kullanıcı eklenemedi: %v", err))
	}

	fmt.Println("✅ 2 kullanıcı başarıyla eklendi.")

	//EXAMPLE 11
	//InsetOrUpdate() Test
	user := mongo.User{
		ID:       4,
		UserName: "player4",
		Email:    "player666@example.com",
	}
	matched, upserted, err := userRepo.InsertOrUpdate(ctx, bson.M{"_id": user.ID}, &user)
	if err != nil {
		panic(fmt.Sprintf("Kullanıcı eklenemedi: %v", err))
	}
	fmt.Printf("matched=%d, upserted=%d\n", matched, upserted)

	//EXAMPLE 12
	//DELETEMANY() Test
	userColl = client.Database("101Okey").Collection("Users")
	userRepo = mongo.MongoRepository[mongo.User]{Collection: userColl}

	filter = mongo.NewQueryBuilder().
		WhereIn("_id", 4, 5).
		Build()
	deletedCount, err := userRepo.DeleteMany(ctx, filter)
	if err != nil {
		panic(fmt.Sprintf("Silme hatası: %v", err))
	}

	fmt.Printf("✅ %d kullanıcı silindi.\n", deletedCount)

	//EXAMPLE 13
	//BulkInsertOrUpdate()

	users = []mongo.User{
		{ID: 42, UserName: "bora", Email: "p1@example.com"},
		{ID: 3, UserName: "player2", Email: "p2@example.com"},
	}

	matched, upserted, err = userRepo.BulkInsertOrUpdate(ctx, users, func(u mongo.User) bson.M {
		return bson.M{"_id": u.ID}
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("matched=%d, upserted=%d\n", matched, upserted)

	//Delete New player2
	filter = mongo.NewQueryBuilder().
		WhereIn("_id", 3).
		Build()
	_, _ = userRepo.DeleteMany(ctx, filter)

	//Update bora Email = bora@mail.com
	user = mongo.User{ID: 42, UserName: "bora", Email: "bora@mail.com"}
	_, _, _ = userRepo.InsertOrUpdate(ctx, bson.M{"_id": user.ID}, &user)

	//EXAMPLE 14
	//LOOKUP() 3 Collections
	builderLookup = mongo.NewAggregateBuilder().
		Lookup("Users", "UserID", "_id", "user").
		Unwind("user").
		Lookup("UserOrders", "user._id", "UserID", "uo").
		UnwindPreserveNull("uo").
		Lookup("Orders", "uo.OrderID", "_id", "order").
		UnwindPreserveNull("order").
		ProjectAliases(
			"UserID", "$UserID",
			"Message", "$Message",
			"DateTime", "$DateTime",
			"UserName", "$user.UserName",
			"Email", "$user.Email",
			"OrderID", "$order._id",
			"OrderCode", "$order.Code",
			"OrderTotal", "$order.Total",
		).
		ProjectKeep("OrderID", "UserName", "OrderCode", "OrderTotal").
		ExcludeID().
		Sort("DateTime", -1).
		Limit(20)
	// Standart çağrı:
	resLookup, err := okeyRepo.Aggregate(ctx, builderLookup)
	// veya büyük dataset için:
	// res, err := okeyRepo.AggregateWithOptions(ctx, builder, options.Aggregate().SetAllowDiskUse(true))

	if err != nil {
		log.Fatal("Aggregate hatası:", err)
	}

	for _, doc := range resLookup {
		orderCode := doc["OrderCode"]
		userName := doc["UserName"]
		orderID := doc["OrderID"]
		orderTotal := doc["OrderTotal"]

		fmt.Printf(
			"🧑 UserName: %s - OrderCode: %s - 📦 OrderID: %v - OrderTotal: %v\n",
			userName, orderCode, orderID, orderTotal,
		)
		//-------------------
	}

	//Example 15
	//DeleteOneSoft
	userColl = client.Database("101Okey").Collection("Users")
	userRepo = mongo.MongoRepository[mongo.User]{Collection: userColl}

	id64 := int64(42)
	filter = bson.M{"_id": id64}

	if err := userRepo.DeleteOneSoft(ctx, filter, "bora"); err != nil {
		log.Println("DeleteOneSoft error:", err)
	} else {
		log.Println("User soft deleted successfully")
	}

	//Example 16
	//DeleteManySoft
	filter = mongo.NumericIDsFilter(43, 44)
	_, err = userRepo.DeleteManySoft(ctx, filter, "bora")
	if err != nil {
		log.Println("DeleteManySoft error:", err)
	} else {
		log.Println("DeleteManySoft successfully")
	}

	//Example 17
	//DeleteOneSoft By Email
	userColl = client.Database("101Okey").Collection("Users")
	userRepo = mongo.MongoRepository[mongo.User]{Collection: userColl}

	filter = bson.M{"_id": id64}

	if err := userRepo.DeleteOneSoft(ctx, filter, "bora"); err != nil {
		log.Println("DeleteOneSoft error:", err)
	} else {
		log.Println("User soft deleted successfully")
	}

	//Example 18 MakePagination(), Count(),
	filter = bson.M{"UserName": "player1"}
	total, err := okeyRepo.Count(ctx, filter)
	if err != nil {
		log.Fatal("Count error:", err)
	}
	fmt.Println("Count():", total)

	// 2) Paging ayarları
	/*	pageSize = int64(3)
		page = 3 // 3. sayfa
		skip := (int64(page) - 1) * pageSize

		pagination = &mongo.Pagination{
			Limit: pageSize,
			Skip:  skip,
		}*/
	currentPage := int64(3)
	pageSize = int64(3)

	newPagination := mongo.MakePagination(currentPage, pageSize)

	// (Opsiyonel) Sıralama: en yeni önce
	sort = &mongo.SortOption{Field: "DateTime", Ascending: false}

	// 3) Sayfadaki kayıtları çek
	items, totalCount, err := okeyRepo.FindWithCount(ctx, filter, sort, newPagination) // default IsDeleted=false
	//items, totalCount, err := okeyRepo.FindWithCount(ctx, filter, sort, &mongo.Pagination{Limit: 3, Skip: 6}) // default IsDeleted=false
	if err != nil {
		log.Fatal("Find error:", err)
	}

	fmt.Printf("Toplam kayıt: %d, Sayfadaki kayıt: %d\n", totalCount, len(items))
	for _, item := range items {
		fmt.Println(item)
	}

	// toplam sayfa (ceiling division)
	totalPages := int64(0)
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}
	fmt.Printf("Sayfa %d/%d\n", currentPage, totalPages)
}
