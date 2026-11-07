package operations

import (
	"bufio"
	"context"
	"mongodumpx/dbcontext"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func BackupCollection(database string, collection string, outputDir string) {
	databaseDir := filepath.Join(outputDir, database)
	if err := os.MkdirAll(databaseDir, 0o755); err != nil {
		panic(err)
	}

	file, err := os.Create(filepath.Join(databaseDir, collection+".json"))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	cursor, err := dbcontext.DB.Database(database).Collection(collection).Find(context.TODO(), bson.D{})
	if err != nil {
		return
	}
	cursor.SetBatchSize(1000)
	defer cursor.Close(context.TODO())

	writer := bufio.NewWriterSize(file, 1<<20)

	if _, err := writer.WriteString("[\n"); err != nil {
		panic(err)
	}

	count := 0
	for cursor.Next(context.TODO()) {
		jsonBytes, err := bson.MarshalExtJSON(cursor.Current, true, false)
		if err != nil {
			panic(err)
		}

		if count > 0 {
			if err := writer.WriteByte(','); err != nil {
				panic(err)
			}
		}
		if _, err := writer.Write(jsonBytes); err != nil {
			panic(err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			panic(err)
		}
		count++
	}
	if err := cursor.Err(); err != nil {
		return
	}

	if _, err := writer.WriteString("]\n"); err != nil {
		panic(err)
	}
	if err := writer.Flush(); err != nil {
		panic(err)
	}
}
