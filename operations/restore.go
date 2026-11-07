package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"mongodumpx/dbcontext"
	"os"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const restoreBatchSize = 1000

// ImportCollection reads the JSON-array backup file and writes it into
// db.coll. The target collection is dropped first (restore = point in
// time). report (optional) is called after each batch with (bytesRead,
// totalBytes). Returns the number of documents inserted.
func ImportCollection(ctx context.Context, filePath string, db string, coll string, report func(done, total int64)) (int64, error) {
	if dbcontext.DB == nil {
		return 0, fmt.Errorf("not connected")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return 0, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer file.Close()

	var total int64
	if info, err := file.Stat(); err == nil {
		total = info.Size()
	}

	// Restore semantics: replace the collection entirely.
	target := dbcontext.DB.Database(db).Collection(coll)
	if err := target.Drop(ctx); err != nil {
		return 0, fmt.Errorf("drop %s.%s: %w", db, coll, err)
	}

	decoder := json.NewDecoder(bufio.NewReaderSize(file, 1<<20))

	tok, err := decoder.Token()
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", filePath, err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("%s: expected JSON array, got %v", filePath, tok)
	}

	insertOpts := options.InsertMany().SetOrdered(false)
	batch := make([]any, 0, restoreBatchSize)
	var count int64

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if _, err := target.InsertMany(ctx, batch, insertOpts); err != nil {
			return fmt.Errorf("inserting into %s.%s: %w", db, coll, err)
		}
		count += int64(len(batch))
		batch = batch[:0]

		if report != nil {
			report(decoder.InputOffset(), total)
		}
		return nil
	}

	for decoder.More() {
		if err := ctx.Err(); err != nil {
			return count, err
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return count, fmt.Errorf("decoding %s: %w", filePath, err)
		}

		var doc bson.Raw
		if err := bson.UnmarshalExtJSON(raw, true, &doc); err != nil {
			return count, fmt.Errorf("parsing document in %s: %w", filePath, err)
		}

		batch = append(batch, doc)
		if len(batch) == restoreBatchSize {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}
	if err := flush(); err != nil {
		return count, err
	}

	if _, err := decoder.Token(); err != nil {
		return count, fmt.Errorf("reading end of %s: %w", filePath, err)
	}

	// Final progress tick.
	if report != nil {
		report(total, total)
	}
	return count, nil
}
