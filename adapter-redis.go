package etler

// import (
// 	"context"
// 	"encoding/json"

// 	"github.com/go-redis/redis/v8"
// )

// // RedisAdapter is an adapter for reading and upserting data in Redis.
// type RedisAdapter struct {
// 	client *redis.Client
// 	key    string
// }

// // Read reads data from Redis using the specified key.
// func (a *RedisAdapter) Read(ctx context.Context, key string) ([]C, error) {
// 	// Get the value from Redis.
// 	val, err := a.client.Get(key).Result()
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Unmarshal the value into a slice of the specified type.
// 	var results []C
// 	err = json.Unmarshal([]byte(val), &results)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return results, nil
// }

// // Upsert upserts data into Redis.
// func (a *RedisAdapter) Upsert(ctx context.Context, data []C) error {
// 	// Marshal the data into JSON.
// 	val, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	// Set the value in Redis.
// 	err = a.client.Set(a.key, val, 0).Err()
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // NewRedisAdapter creates a new RedisAdapter.
// func NewRedisAdapter(client *redis.Client, key string) *RedisAdapter {
// 	return &RedisAdapter{client: client, key: key}
// }
