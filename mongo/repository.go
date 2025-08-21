package mongo

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

// OpType: Enum benzeri sorgu operatörleri
var OpTypes = newOpTypes()

func newOpTypes() *opTypes {
	return &opTypes{
		Eq:    "$eq",
		Gt:    "$gt",
		Gte:   "$gte",
		Lt:    "$lt",
		Lte:   "$lte",
		Ne:    "$ne",
		In:    "$in",
		Nin:   "$nin",
		Regex: "$regex",
	}
}

type opTypes struct {
	Eq    OpType
	Gt    OpType
	Gte   OpType
	Lt    OpType
	Lte   OpType
	Ne    OpType
	In    OpType
	Nin   OpType
	Regex OpType
}

type OpType string

// Mock Data ve Unuit Test icin Interface Repository
type Reader[T any] interface {
	// Default: IsDeleted=false (aktif kayıtlar). İstersen &[]bool{true}[0] ile silinmişleri çek.
	Find(ctx context.Context, filter bson.M, sort *SortOption, pagination *Pagination, isDeleted ...*bool) ([]T, error)
	FindOne(ctx context.Context, filter bson.M, isDeleted ...*bool) (*T, error)
	FindWithCount(ctx context.Context, filter bson.M, sort *SortOption, pagination *Pagination, isDeleted ...*bool) ([]T, int64, error)

	// Paging/raporlama için şart
	Count(ctx context.Context, filter bson.M, isDeleted ...*bool) (int64, error)
}

type Writer[T any] interface {
	// Insert’ler
	Insert(ctx context.Context, doc *T) (interface{}, error)
	BulkInsert(ctx context.Context, docs []T) error
	BulkInsertWithIDs(ctx context.Context, docs []T) ([]interface{}, error)

	// Upsert (replace semantiği)
	InsertOrUpdate(ctx context.Context, filter bson.M, doc *T) (matched, upserted int64, err error)
	BulkInsertOrUpdate(ctx context.Context, docs []T, filterFn func(doc T) bson.M) (matched, upserted int64, err error)

	// Upsert ($set semantiği – alan bazlı)
	BulkInsertOrUpdateForFields(ctx context.Context, docs []T, filterFn func(doc T) bson.M, setFn func(doc T) bson.M) (matched, upserted int64, err error)

	// Update’ler
	UpdateOne(ctx context.Context, filter bson.M, update bson.M, upsert bool) error
	BulkUpdate(ctx context.Context, filter bson.M, update bson.M, upsert bool) (int64, error)

	// (Opsiyonel ama çok faydalı) Güncellenmiş dokümanı döndür
	FindOneAndUpdate(ctx context.Context, filter bson.M, update bson.M, returnAfter bool, upsert bool) (*T, error)

	// Delete’ler
	DeleteOne(ctx context.Context, filter bson.M) error
	DeleteMany(ctx context.Context, filter bson.M) (int64, error)

	// Soft-delete (back-office için ideal)
	DeleteOneSoft(ctx context.Context, filter bson.M, deletedBy string) error
	DeleteManySoft(ctx context.Context, filter bson.M, deletedBy string) (int64, error)
}

type Aggregator[T any] interface {
	Aggregate(ctx context.Context, builder *AggregateBuilder) ([]bson.M, error)
	AggregateWithOptions(ctx context.Context, builder *AggregateBuilder, opts *options.AggregateOptions) ([]bson.M, error)
}

type Repository[T any] interface {
	Reader[T]
	Writer[T]
	Aggregator[T]
}

//-------------Interface Description Finished--------

// Query: Tekli sorgu
type Query struct {
	Field string
	Op    OpType
	Value interface{}
}

func (q Query) ToBSON() bson.M {
	return bson.M{q.Field: bson.M{string(q.Op): q.Value}}
}

// QueryBuilder: Zincirleme filtre inşa aracı
type QueryBuilder struct {
	conditions []bson.M
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{conditions: []bson.M{}}
}

/*func (qb *QueryBuilder) Where(field string, op OpType, value interface{}) *QueryBuilder {
	qb.conditions = append(qb.conditions, Query{field, op, value}.ToBSON())
	return qb
}*/

// Biden fazla where pes pese kullanilabilsin diye Merge ediliyor..
func (qb *QueryBuilder) Where(field string, op OpType, value interface{}) *QueryBuilder {
	// Var olan condition’ı bul & merge et
	for i, c := range qb.conditions {
		if cond, ok := c[field]; ok {
			// cond genelde bson.M
			m := cond.(bson.M)
			m[string(op)] = value
			qb.conditions[i] = bson.M{field: m}
			return qb
		}
	}
	qb.conditions = append(qb.conditions, bson.M{field: bson.M{string(op): value}})
	return qb
}

// WhereIn: Belirtilen alanda $in operatörü ile çoklu değer sorgusu
// WhereIn: $in operatörü, tek argüman slice/bson.A ise flatten eder
func (qb *QueryBuilder) WhereIn(field string, values ...interface{}) *QueryBuilder {
	// flatten
	if len(values) == 1 {
		switch v := values[0].(type) {
		case bson.A:
			values = []interface{}(v)
		case []interface{}:
			values = v
		case []primitive.ObjectID:
			tmp := make([]interface{}, len(v))
			for i, x := range v {
				tmp[i] = x
			}
			values = tmp
		}
	}

	// mevcut condition varsa merge et
	for i, c := range qb.conditions {
		if cond, ok := c[field]; ok {
			m := cond.(bson.M)
			m[string(OpTypes.In)] = values
			qb.conditions[i] = bson.M{field: m}
			return qb
		}
	}

	qb.conditions = append(qb.conditions, bson.M{
		field: bson.M{string(OpTypes.In): values},
	})
	return qb
}

func (qb *QueryBuilder) And(field string, op OpType, value interface{}) *QueryBuilder {
	return qb.Where(field, op, value)
}

func (qb *QueryBuilder) Or(orConditions ...Query) *QueryBuilder {
	var orBSON []bson.M
	for _, q := range orConditions {
		orBSON = append(orBSON, q.ToBSON())
	}
	qb.conditions = append(qb.conditions, bson.M{"$or": orBSON})
	return qb
}

func (qb *QueryBuilder) Not(query Query) *QueryBuilder {
	qb.conditions = append(qb.conditions, bson.M{query.Field: bson.M{"$not": bson.M{string(query.Op): query.Value}}})
	return qb
}

func (qb *QueryBuilder) Build() bson.M {
	if len(qb.conditions) == 0 {
		return bson.M{}
	}
	if len(qb.conditions) == 1 {
		return qb.conditions[0]
	}
	return bson.M{"$and": qb.conditions}
}

// ComplexQuery: And, Or, Not kombinasyonları için
func AndQuery(queries ...bson.M) bson.M {
	return bson.M{"$and": queries}
}

func OrQuery(queries ...bson.M) bson.M {
	return bson.M{"$or": queries}
}

func NotQuery(query bson.M) bson.M {
	return bson.M{"$not": query}
}

// SortOption ve Pagination

type SortOption struct {
	Field     string
	Ascending bool
}

type Pagination struct {
	Limit int64
	Skip  int64
}

// MongoRepository - generic repo
type MongoRepository[T any] struct {
	Collection *mongo.Collection
}

// InsertOne
/*func (r *MongoRepository[T]) Insert(ctx context.Context, doc *T) error {
	_, err := r.Collection.InsertOne(ctx, doc)
	return err
}*/

func (r *MongoRepository[T]) Insert(ctx context.Context, doc *T) (interface{}, error) {
	res, err := r.Collection.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	return res.InsertedID, nil
}

// InsertOrUpdate: belge varsa replace, yoksa insert (upsert)
func (r *MongoRepository[T]) InsertOrUpdate(ctx context.Context, filter bson.M, doc *T) (matched, upserted int64, err error) {
	res, err := r.Collection.ReplaceOne(ctx, filter, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return 0, 0, err
	}
	var ups int64
	if res.UpsertedID != nil {
		ups = 1
	}
	return res.MatchedCount, ups, nil
}

// BulkInsert
func (r *MongoRepository[T]) BulkInsert(ctx context.Context, docs []T) error {
	var insertDocs []interface{}
	for _, d := range docs {
		insertDocs = append(insertDocs, d)
	}
	_, err := r.Collection.InsertMany(ctx, insertDocs)
	return err
}

// BulkInsert With Inserted IDS
func (r *MongoRepository[T]) BulkInsertWithIDs(ctx context.Context, docs []T) ([]interface{}, error) {
	var insertDocs []interface{}
	for _, d := range docs {
		insertDocs = append(insertDocs, d)
	}

	res, err := r.Collection.InsertMany(ctx, insertDocs)
	if err != nil {
		return nil, err
	}
	return res.InsertedIDs, nil
}

// BulkInsertOrUpdateSet: docs için toplu upsert ($set semantiği)
// setFn: her doc için setlenecek alanları üretir
func (r *MongoRepository[T]) BulkInsertOrUpdateForFields(
	ctx context.Context,
	docs []T,
	filterFn func(doc T) bson.M,
	setFn func(doc T) bson.M,
) (matched, upserted int64, err error) {

	if len(docs) == 0 {
		return 0, 0, nil
	}

	models := make([]mongo.WriteModel, 0, len(docs))
	for _, d := range docs {
		u := mongo.NewUpdateOneModel().
			SetFilter(filterFn(d)).
			SetUpdate(bson.M{"$set": setFn(d)}).
			SetUpsert(true)
		models = append(models, u)
	}

	res, err := r.Collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, 0, err
	}
	return res.MatchedCount, int64(len(res.UpsertedIDs)), nil
}

// BulkInsertOrUpdate: docs için toplu upsert (Replace semantiği)
// filterFn: her doc için upsert filtresini üretir (ör. {"_id": doc.ID})
func (r *MongoRepository[T]) BulkInsertOrUpdate(
	ctx context.Context,
	docs []T,
	filterFn func(doc T) bson.M,
) (matched, upserted int64, err error) {

	if len(docs) == 0 {
		return 0, 0, nil
	}

	models := make([]mongo.WriteModel, 0, len(docs))
	for _, d := range docs {
		m := mongo.NewReplaceOneModel().
			SetFilter(filterFn(d)).
			SetReplacement(d).
			SetUpsert(true)
		models = append(models, m)
	}

	res, err := r.Collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, 0, err
	}
	return res.MatchedCount, int64(len(res.UpsertedIDs)), nil
}

// FindMany
/*func (r *MongoRepository[T]) Find(ctx context.Context, filter bson.M, sort *SortOption, pagination *Pagination) ([]T, error) {
	findOptions := options.Find()
	if sort != nil {
		direction := 1
		if !sort.Ascending {
			direction = -1
		}
		findOptions.SetSort(bson.D{{Key: sort.Field, Value: direction}})
	}
	if pagination != nil {
		findOptions.SetLimit(pagination.Limit)
		findOptions.SetSkip(pagination.Skip)
	}

	cursor, err := r.Collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	for cursor.Next(ctx) {
		var item T
		if err := cursor.Decode(&item); err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}*/

// FindMany
// Find: opsiyonel IsDeleted predicate (variadic ...*bool)
// isDeleted verilirse ve nil değilse filtreye IsDeleted koşulu eklenir.
// Default Sadece aktif (silinmemiş) kayıtlar getirilir.
func (r *MongoRepository[T]) Find(
	ctx context.Context,
	filter bson.M,
	sort *SortOption,
	pagination *Pagination,
	isDeleted ...*bool, // ✅ opsiyonel predicate
) ([]T, error) {

	// Opsiyonel IsDeleted koşulu
	if len(isDeleted) > 0 && isDeleted[0] != nil {
		filter["IsDeleted"] = *isDeleted[0]
	} else {
		filter["IsDeleted"] = false
	}

	findOptions := options.Find()
	if sort != nil {
		direction := 1
		if !sort.Ascending {
			direction = -1
		}
		findOptions.SetSort(bson.D{{Key: sort.Field, Value: direction}})
	}
	if pagination != nil {
		findOptions.SetLimit(pagination.Limit)
		findOptions.SetSkip(pagination.Skip)
	}

	cursor, err := r.Collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	for cursor.Next(ctx) {
		var item T
		if err := cursor.Decode(&item); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	// 🔎 döngü sonrası cursor hatasını kontrol et
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// FindOne
/*func (r *MongoRepository[T]) FindOne(ctx context.Context, filter bson.M) (*T, error) {
	result := r.Collection.FindOne(ctx, filter)
	if result.Err() != nil {
		return nil, result.Err()
	}
	var item T
	if err := result.Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}*/
// FindOne With Optional IsDeleted Parameter
// Default Sadece aktif (silinmemiş) kayıdi getirir.
func (r *MongoRepository[T]) FindOne(
	ctx context.Context,
	filter bson.M,
	isDeleted ...*bool, // ✅ opsiyonel predicate
) (*T, error) {
	// Eğer parametre verilmişse ve nil değilse filtreye uygula
	if len(isDeleted) > 0 && isDeleted[0] != nil {
		filter["IsDeleted"] = *isDeleted[0]
	} else {
		filter["IsDeleted"] = false
	}

	result := r.Collection.FindOne(ctx, filter)
	if result.Err() != nil {
		return nil, result.Err()
	}

	var item T
	if err := result.Decode(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateOne veya Upsert
func (r *MongoRepository[T]) UpdateOne(ctx context.Context, filter bson.M, update bson.M, upsert bool) error {
	opts := options.Update().SetUpsert(upsert)
	res, err := r.Collection.UpdateOne(ctx, filter, bson.M{"$set": update}, opts)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 && !upsert {
		return errors.New("no documents matched")
	}
	return nil
}

// BulkUpdate - çoklu belge güncelleme
func (r *MongoRepository[T]) BulkUpdate(ctx context.Context, filter bson.M, update bson.M, upsert bool) (int64, error) {
	opts := options.Update().SetUpsert(upsert)
	res, err := r.Collection.UpdateMany(ctx, filter, bson.M{"$set": update}, opts)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// DeleteOne
func (r *MongoRepository[T]) DeleteOne(ctx context.Context, filter bson.M) error {
	res, err := r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return errors.New("no documents deleted")
	}
	return nil
}

// DeleteMany - çoklu document silme
func (r *MongoRepository[T]) DeleteMany(ctx context.Context, filter bson.M) (int64, error) {
	res, err := r.Collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	if res.DeletedCount == 0 {
		return 0, errors.New("no documents deleted")
	}
	return res.DeletedCount, nil
}

func (r *MongoRepository[T]) DeleteOneSoft(ctx context.Context, filter bson.M, deletedBy string) error {
	if filter == nil {
		filter = bson.M{}
	}
	// Don't target already-deleted docs unless caller explicitly overrides
	if _, ok := filter["IsDeleted"]; !ok {
		filter["IsDeleted"] = bson.M{"$ne": true}
	}

	now := time.Now().UTC()
	update := bson.M{"$set": bson.M{
		"IsDeleted": true,
		"DeletedAt": &now, // pointer field in your model
		"DeletedBy": deletedBy,
	}}

	res, err := r.Collection.UpdateOne(ctx, filter, update, options.Update().SetUpsert(false))
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return errors.New("no documents matched")
	}
	if res.ModifiedCount == 0 {
		return errors.New("document matched but not modified (maybe already soft-deleted)")
	}
	return nil
}

func (r *MongoRepository[T]) DeleteManySoft(ctx context.Context, filter bson.M, deletedBy string) (int64, error) {
	if filter == nil {
		filter = bson.M{}
	}
	if _, ok := filter["IsDeleted"]; !ok {
		filter["IsDeleted"] = bson.M{"$ne": true}
	}

	now := time.Now().UTC()
	update := bson.M{"$set": bson.M{
		"IsDeleted": true,
		"DeletedAt": &now,
		"DeletedBy": deletedBy,
	}}

	res, err := r.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	if res.MatchedCount == 0 {
		return 0, errors.New("no documents matched")
	}
	if res.ModifiedCount == 0 {
		return 0, errors.New("documents matched but not modified (maybe already soft-deleted)")
	}
	return res.ModifiedCount, nil
}

// filter := NumericIDsFilter(42, 43, 44)
func NumericIDsFilter(ids ...int64) bson.M {
	in := make([]any, 0, len(ids)*2)
	for _, id := range ids {
		in = append(in, int32(id), int64(id)) // match both NumberInt and NumberLong
	}
	return bson.M{"_id": bson.M{"$in": in}}
}

// Aggregate
/*func (r *MongoRepository[T]) Aggregate(ctx context.Context, pipeline mongo.Pipeline) ([]bson.M, error) {
	cursor, err := r.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}*/

func (r *MongoRepository[T]) Aggregate(ctx context.Context, builder *AggregateBuilder) ([]bson.M, error) {
	pipeline := builder.Build()

	cursor, err := r.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// Helper: String to ObjectID
func ToObjectID(idStr string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		log.Fatal("Invalid ObjectID:", err)
	}
	return id
}

// Helper: String to Time (RFC3339)
func ToTimeRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		log.Fatal("Invalid date format:", err)
	}
	return t
}

// Aggregate Query Chain
type AggregateBuilder struct {
	pipeline mongo.Pipeline
}

func NewAggregateBuilder() *AggregateBuilder {
	return &AggregateBuilder{pipeline: mongo.Pipeline{}}
}

// $match
func (ab *AggregateBuilder) Match(filter bson.M) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$match", Value: filter}})
	return ab
}

// $group
func (ab *AggregateBuilder) Group(id interface{}, fields bson.M) *AggregateBuilder {
	group := bson.D{{Key: "$group", Value: bson.M{"_id": id}}}
	for k, v := range fields {
		group[0].Value.(bson.M)[k] = v
	}
	ab.pipeline = append(ab.pipeline, group)
	return ab
}

// $sort
func (ab *AggregateBuilder) Sort(field string, direction int) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$sort", Value: bson.D{{Key: field, Value: direction}}}})
	return ab
}

type SortField struct {
	Field string
	Asc   bool
}
type SortOptions struct{ Fields []SortField }

func Sorts(so *SortOptions) bson.D {
	if so == nil {
		return nil
	}
	d := bson.D{}
	for _, f := range so.Fields {
		v := 1
		if !f.Asc {
			v = -1
		}
		d = append(d, bson.E{Key: f.Field, Value: v})
	}
	return d
}

// $project
func (ab *AggregateBuilder) Project(fields bson.M) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$project", Value: fields}})
	return ab
}

// $limit
func (ab *AggregateBuilder) Limit(n int64) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$limit", Value: n}})
	return ab
}

// $skip
func (ab *AggregateBuilder) Skip(n int64) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$skip", Value: n}})
	return ab
}

// $unwind
func (ab *AggregateBuilder) Unwind(field string) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$unwind", Value: "$" + field}})
	return ab
}

// $lookup
func (ab *AggregateBuilder) Lookup(from, localField, foreignField, as string) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$lookup", Value: bson.M{
		"from":         from,
		"localField":   localField,
		"foreignField": foreignField,
		"as":           as,
	}}})
	return ab
}

// $count
func (ab *AggregateBuilder) Count(alias string) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$count", Value: alias}})
	return ab
}

// Build
func (ab *AggregateBuilder) Build() mongo.Pipeline {
	return ab.pipeline
}

// === AggregateBuilder Helpers Multi Relation ===

// $unwind (preserveNullAndEmptyArrays: true)
func (ab *AggregateBuilder) UnwindPreserveNull(field string) *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$unwind", Value: bson.M{
		"path": "$" + field, "preserveNullAndEmptyArrays": true,
	}}})
	return ab
}

// $project: verilen alanları 1 yap (bsonsuz kısa yazım)
func (ab *AggregateBuilder) ProjectKeep(fields ...string) *AggregateBuilder {
	if len(fields) == 0 {
		return ab
	}
	m := bson.M{}
	for _, f := range fields {
		m[f] = 1
	}
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$project", Value: m}})
	return ab
}

// $addFields: alias = expr çiftlerini tek seferde ekle
// Örn: ProjectAliases("UserName","$user.UserName","Email","$user.Email")
func (ab *AggregateBuilder) ProjectAliases(pairs ...string) *AggregateBuilder {
	if len(pairs)%2 != 0 {
		return ab
	} // güvenlik: çift olmalı
	m := bson.M{}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$addFields", Value: m}})
	return ab
}

// _id'yi gizlemek için kısa yol
func (ab *AggregateBuilder) ExcludeID() *AggregateBuilder {
	ab.pipeline = append(ab.pipeline, bson.D{{Key: "$project", Value: bson.M{"_id": 0}}})
	return ab
}

// === Aggregate with options (opsiyonel) ===
func (r *MongoRepository[T]) AggregateWithOptions(
	ctx context.Context,
	builder *AggregateBuilder,
	opts *options.AggregateOptions,
) ([]bson.M, error) {
	cursor, err := r.Collection.Aggregate(ctx, builder.Build(), opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *MongoRepository[T]) Count(ctx context.Context, filter bson.M, isDeleted ...*bool) (int64, error) {
	if len(isDeleted) > 0 && isDeleted[0] != nil {
		filter["IsDeleted"] = *isDeleted[0]
	} else {
		filter["IsDeleted"] = false
	}
	return r.Collection.CountDocuments(ctx, filter)
}

// page: 1-based (1,2,3,...)  | pageSize: >0
func MakePagination(page, pageSize int64) *Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	skip := (page - 1) * pageSize
	return &Pagination{Limit: pageSize, Skip: skip}
}

// ortak helper (mutates)
func ensureActiveFilter(filter bson.M, isDeleted ...*bool) {
	if len(isDeleted) > 0 && isDeleted[0] != nil {
		filter["IsDeleted"] = *isDeleted[0]
		return
	}
	if _, ok := filter["IsDeleted"]; !ok {
		filter["IsDeleted"] = false
	}
}

// FindWithCount: aynı filtreyle toplam sayıyı ve sayfa verisini döndürür
// Default IsDeleted = false
func (r *MongoRepository[T]) FindWithCount(
	ctx context.Context,
	filter bson.M,
	sort *SortOption,
	pagination *Pagination,
	isDeleted ...*bool,
) ([]T, int64, error) {

	// 👉 Dışarıdaki filter’ı klonlamadan, aynı referans üzerinde çalış
	ensureActiveFilter(filter, isDeleted...)

	// 1) Toplam
	total, err := r.Collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 2) Sayfa verisi (ister r.Find ile, ister direkt Collection.Find ile)
	items, err := r.Find(ctx, filter, sort, pagination, isDeleted...)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

/* Mongo Update Collection
db.Users.updateMany(
    {},
    {
        $set: {
            IsDeleted: false,
            DeletedBy: null,
            DeletedAt: null
        }
    }
)
*/
