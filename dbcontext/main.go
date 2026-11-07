package dbcontext

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var DB *mongo.Client

func InitDB() *mongo.Client {
	options := options.Client()
	options.ApplyURI("mongodb://127.0.0.1:27017")

	client, err := mongo.Connect(options)
	if err != nil {
		panic(err)
	}
	DB = client
	return client
}
