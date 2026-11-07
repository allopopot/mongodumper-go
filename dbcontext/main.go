package dbcontext

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var DB *mongo.Client

// Connect dials the given URI, verifies connectivity with a ping, and
// stores the client in the package-level DB variable.
func Connect(ctx context.Context, uri string) error {
	if DB != nil {
		_ = DB.Disconnect(ctx)
		DB = nil
	}

	opts := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(opts)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return fmt.Errorf("ping: %w", err)
	}

	DB = client
	return nil
}

// Disconnect closes the current connection, if any.
func Disconnect(ctx context.Context) error {
	if DB == nil {
		return nil
	}
	err := DB.Disconnect(ctx)
	DB = nil
	return err
}

// ListDatabases returns the names of all non-system databases, sorted.
func ListDatabases(ctx context.Context) ([]string, error) {
	if DB == nil {
		return nil, fmt.Errorf("not connected")
	}

	res, err := DB.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}

	system := map[string]bool{"admin": true, "config": true, "local": true}
	out := make([]string, 0, len(res))
	for _, name := range res {
		if !system[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// ListCollections returns the names of all collections in the given
// database, sorted.
func ListCollections(ctx context.Context, database string) ([]string, error) {
	if DB == nil {
		return nil, fmt.Errorf("not connected")
	}

	names, err := DB.Database(database).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("list collections in %s: %w", database, err)
	}
	sort.Strings(names)
	return names, nil
}
