package etler

// import (
// 	"context"

// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"
// )

// // Adapter is an interface for reading and upserting data.
// type Adapter [C any]interface {
// 	Read(ctx context.Context, query interface{}) ([]C, error)
// 	Upsert(ctx context.Context, data []C) error
// }

// // MongoDBAdapter is an adapter for reading and upserting data in MongoDB.
// type MongoDBAdapter [C any]struct {
// 	client     *mongo.Client
// 	database   string
// 	collection string
// }

// // Read reads data from MongoDB using the specified query.
// func (a *MongoDBAdapter[C any]) Read(ctx context.Context, query interface{}) ([]C, error) {
// 	// Set up the find options.
// 	findOptions := options.Find()

// 	// Execute the find query and retrieve the results.
// 	cur, err := a.client.Database(a.database).Collection(a.collection).Find(ctx, query, findOptions)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer cur.Close(ctx)

// 	// Unmarshal the search results into a slice of the specified type.
// 	var results []C
// 	for cur.Next(ctx) {
// 		var result C
// 		err := cur.Decode(&result)
// 		if err != nil {
// 			return nil, err
// 		}
// 		results = append(results, result)
// 	}
// 	if err := cur.Err(); err != nil {
// 		return nil, err
// 	}

// 	return results, nil
// }

// // Upsert upserts data into MongoDB.
// func (a *MongoDBAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Get a handle to the collection.
// 	col := a.client.Database(a.database).Collection(a.collection)

// 	// Iterate through the data and upsert each item.
// 	for _, item := range data {
// 		_, err := col.ReplaceOne(ctx, item, item, options.Replace().SetUpsert(true))
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }
