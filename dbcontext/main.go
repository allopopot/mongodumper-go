package dbcontext

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var DB *mongo.Client

func InitDB(uri string) (*mongo.Client, error) {
	options := options.Client()
	options.ApplyURI(uri)

	client, err := mongo.Connect(options)
	if err != nil {
		return nil, err
	} else {
		if err = client.Ping(context.TODO(), readpref.Primary()); err != nil {
			return nil, err
		} else {
			DB = client
			return client, nil
		}
	}
}
