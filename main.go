package main

import (
	"context"
	"mongodumpx/dbcontext"
)

func main() {
	client := dbcontext.InitDB()

	client.Disconnect(context.TODO())
}
