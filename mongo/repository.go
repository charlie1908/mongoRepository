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
	Find(ctx context.Context, filter bson.M, sort *SortOption, pagination *Pagination) ([]T, error)
	FindOne(ctx context.Context, filter bson.M) (*T, error)
}

type Writer[T any] interface {
	Insert(ctx context.Context, doc *T) error
	BulkInsert(ctx context.Context, docs []T) error
	UpdateOne(ctx context.Context, filter bson.M, update bson.M, upsert bool) error
	BulkUpdate(ctx context.Context, filter bson.M, update bson.M) (int64, error)
	DeleteOne(ctx context.Context, filter bson.M) error
}

type Aggregator[T any] interface {
	Aggregate(ctx context.Context, builder *AggregateBuilder) ([]bson.M, error)
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
func (r *MongoRepository[T]) Insert(ctx context.Context, doc *T) error {
	_, err := r.Collection.InsertOne(ctx, doc)
	return err
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

// FindMany
func (r *MongoRepository[T]) Find(ctx context.Context, filter bson.M, sort *SortOption, pagination *Pagination) ([]T, error) {
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
}

// FindOne
func (r *MongoRepository[T]) FindOne(ctx context.Context, filter bson.M) (*T, error) {
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
func (r *MongoRepository[T]) BulkUpdate(ctx context.Context, filter bson.M, update bson.M) (int64, error) {
	res, err := r.Collection.UpdateMany(ctx, filter, bson.M{"$set": update})
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
