package driver

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDriver struct {
	client *mongo.Client
	db     *mongo.Database
}

func (d *MongoDriver) Name() string { return "mongodb" }

func (d *MongoDriver) Connect(ctx context.Context, dsn string) error {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(dsn))
	if err != nil {
		return fmt.Errorf("mongo: connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		client.Disconnect(ctx)
		return fmt.Errorf("mongo: ping: %w", err)
	}

	dbName := extractMongoDBName(dsn)
	if dbName == "" {
		return fmt.Errorf("mongo: database name required in URI (e.g., mongodb://localhost:27017/mydb)")
	}

	d.client = client
	d.db = client.Database(dbName)
	return nil
}

func extractMongoDBName(dsn string) string {
	slashes := 0
	start := -1
	for i, ch := range dsn {
		if ch == '/' {
			slashes++
			if slashes == 3 {
				start = i + 1
				break
			}
		}
	}
	if start < 0 || start >= len(dsn) {
		return ""
	}
	name := dsn[start:]
	for i, ch := range name {
		if ch == '?' {
			return name[:i]
		}
	}
	return name
}

func (d *MongoDriver) ExecSQL(_ context.Context, _ string) error {
	return fmt.Errorf("mongo: SQL not supported; use JSON or Go seeds")
}

func (d *MongoDriver) InsertJSON(ctx context.Context, coll string, records []map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("mongo: not connected")
	}
	if len(records) == 0 {
		return nil
	}
	docs := make([]interface{}, len(records))
	for i, rec := range records {
		doc := bson.D{}
		for k, v := range rec {
			doc = append(doc, bson.E{Key: k, Value: v})
		}
		docs[i] = doc
	}
	_, err := d.db.Collection(coll).InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("mongo: insert into %s: %w", coll, err)
	}
	return nil
}

func (d *MongoDriver) DeleteJSON(ctx context.Context, coll string, filter map[string]interface{}) error {
	if d.db == nil {
		return fmt.Errorf("mongo: not connected")
	}
	if len(filter) == 0 {
		return nil
	}
	f := bson.D{}
	for k, v := range filter {
		f = append(f, bson.E{Key: k, Value: v})
	}
	_, err := d.db.Collection(coll).DeleteMany(ctx, f)
	if err != nil {
		return fmt.Errorf("mongo: delete from %s: %w", coll, err)
	}
	return nil
}

func (d *MongoDriver) Truncate(ctx context.Context, colls ...string) error {
	if d.db == nil {
		return fmt.Errorf("mongo: not connected")
	}
	for _, c := range colls {
		if err := d.db.Collection(c).Drop(ctx); err != nil {
			return fmt.Errorf("mongo: drop %s: %w", c, err)
		}
	}
	return nil
}

// --- Version tracking ---

func (d *MongoDriver) CreateVersionTable(ctx context.Context) error {
	coll := d.db.Collection("seeder_versions")
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (d *MongoDriver) GetAppliedVersions(ctx context.Context) ([]AppliedSeed, error) {
	coll := d.db.Collection("seeder_versions")
	cursor, err := coll.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "version", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var seeds []AppliedSeed
	for cursor.Next(ctx) {
		var doc struct {
			Version   int64     `bson:"version"`
			Name      string    `bson:"seed_name"`
			Dirty     bool      `bson:"dirty"`
			WhyDirty  string    `bson:"why_dirty"`
			AppliedAt time.Time `bson:"applied_at"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		seeds = append(seeds, AppliedSeed{
			Version: doc.Version, Name: doc.Name,
			Dirty: doc.Dirty, WhyDirty: doc.WhyDirty, AppliedAt: doc.AppliedAt,
		})
	}
	return seeds, cursor.Err()
}

func (d *MongoDriver) RecordVersion(ctx context.Context, version int64, name string, dirty bool, whyDirty string) error {
	_, err := d.db.Collection("seeder_versions").InsertOne(ctx, bson.D{
		{Key: "version", Value: version},
		{Key: "seed_name", Value: name},
		{Key: "dirty", Value: dirty},
		{Key: "why_dirty", Value: whyDirty},
		{Key: "applied_at", Value: time.Now().UTC()},
	})
	return err
}

func (d *MongoDriver) SetDirty(ctx context.Context, version int64, dirty bool, whyDirty string) error {
	_, err := d.db.Collection("seeder_versions").UpdateOne(ctx,
		bson.D{{Key: "version", Value: version}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "dirty", Value: dirty}, {Key: "why_dirty", Value: whyDirty}}}},
	)
	return err
}

func (d *MongoDriver) RemoveVersion(ctx context.Context, version int64) error {
	_, err := d.db.Collection("seeder_versions").DeleteOne(ctx,
		bson.D{{Key: "version", Value: version}})
	return err
}

func (d *MongoDriver) Close(ctx context.Context) error {
	if d.client != nil {
		return d.client.Disconnect(ctx)
	}
	return nil
}
