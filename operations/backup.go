package operations

import (
	"bufio"
	"context"
	"fmt"
	"mongodumpx/dbcontext"
	"os"
	"path/filepath"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ExportCollection writes every document of db.coll to
// outputDir/<db>/<coll>.json as a JSON array. report (optional) is called
// after each document with (done, total) where total is 0 if unknown.
// Returns the number of documents written.
func ExportCollection(ctx context.Context, db string, coll string, outputDir string, report func(done, total int64)) (int64, error) {
	if dbcontext.DB == nil {
		return 0, fmt.Errorf("not connected")
	}

	databaseDir := filepath.Join(outputDir, db)
	if err := os.MkdirAll(databaseDir, 0o755); err != nil {
		return 0, fmt.Errorf("create dir %s: %w", databaseDir, err)
	}

	file, err := os.Create(filepath.Join(databaseDir, coll+".json"))
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	cursor, err := dbcontext.DB.Database(db).Collection(coll).Find(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("find in %s.%s: %w", db, coll, err)
	}
	defer cursor.Close(ctx)

	// Best-effort document total for progress reporting.
	var total int64
	if n, err := dbcontext.DB.Database(db).Collection(coll).EstimatedDocumentCount(ctx); err == nil {
		total = n
	}

	writer := bufio.NewWriterSize(file, 1<<20)

	if _, err := writer.WriteString("[\n"); err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}

	var done int64
	for cursor.Next(ctx) {
		if err := ctx.Err(); err != nil {
			return done, err
		}

		jsonBytes, err := bson.MarshalExtJSON(cursor.Current, true, false)
		if err != nil {
			return done, fmt.Errorf("marshal document: %w", err)
		}

		if done > 0 {
			if err := writer.WriteByte(','); err != nil {
				return done, fmt.Errorf("write: %w", err)
			}
		}
		if _, err := writer.Write(jsonBytes); err != nil {
			return done, fmt.Errorf("write: %w", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			return done, fmt.Errorf("write: %w", err)
		}

		done++
		if report != nil {
			report(done, total)
		}
	}
	if err := cursor.Err(); err != nil {
		return done, fmt.Errorf("cursor %s.%s: %w", db, coll, err)
	}

	if _, err := writer.WriteString("]\n"); err != nil {
		return done, fmt.Errorf("write: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return done, fmt.Errorf("flush: %w", err)
	}
	return done, nil
}
