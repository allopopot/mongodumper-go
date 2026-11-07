package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"mongodumpx/dbcontext"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const restoreBatchSize = 1000

func RestoreCollection(filePath string, database string, collection string) {
	file, err := os.Open(filepath.Join(filePath))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decoder := json.NewDecoder(bufio.NewReaderSize(file, 1<<20))

	tok, err := decoder.Token()
	if err != nil {
		panic(fmt.Errorf("reading %s: %w", filePath, err))
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		panic(fmt.Errorf("%s: expected JSON array, got %v", filePath, tok))
	}

	coll := dbcontext.DB.Database(database).Collection(collection)
	insertOpts := options.InsertMany().SetOrdered(false)

	batch := make([]any, 0, restoreBatchSize)
	count := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if _, err := coll.InsertMany(context.TODO(), batch, insertOpts); err != nil {
			panic(fmt.Errorf("inserting into %s.%s: %w", database, collection, err))
		}
		count += len(batch)
		batch = batch[:0]
	}

	for decoder.More() {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			panic(fmt.Errorf("decoding %s: %w", filePath, err))
		}

		var doc bson.Raw
		if err := bson.UnmarshalExtJSON(raw, true, &doc); err != nil {
			panic(fmt.Errorf("parsing document in %s: %w", filePath, err))
		}

		batch = append(batch, doc)
		if len(batch) == restoreBatchSize {
			flush()
		}
	}
	flush()

	if _, err := decoder.Token(); err != nil {
		panic(fmt.Errorf("reading end of %s: %w", filePath, err))
	}
}
