package etler

// // SQLAdapter is an adapter for reading and upserting data in a SQL database.
// type SQLAdapter[C any] struct {
// 	db     *sql.DB
// 	table  string
// 	driver string
// }

// // Read reads data from the SQL database using the specified query.
// func (a *SQLAdapter[C any]) Read(ctx context.Context, query interface{}) ([]C, error) {
// 	// Execute the query and get the rows.
// 	rows, err := a.db.Query(query)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	// Scan the rows into a slice of the specified type.
// 	var results []C
// 	for rows.Next() {
// 		var result C
// 		err := rows.Scan(&result)
// 		if err != nil {
// 			return nil, err
// 		}
// 		results = append(results, result)
// 	}

// 	return results, nil
// }

// // Upsert upserts data into the SQL database.
// func (a *SQLAdapter[C any]) Upsert(ctx context.Context, data []C) error {
// 	// Begin a transaction.
// 	tx, err := a.db.Begin()
// 	if err != nil {
// 		return err
// 	}

// 	// Prepare the upsert statement.
// 	stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s VALUES (?)", a.table))
// 	if err != nil {
// 		return err
// 	}
// 	defer stmt.Close()

// 	// Execute the upsert statement for each item in the slice.
// 	for _, item := range data {
// 		_, err := stmt.Exec(item)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// Commit the transaction.
// 	err = tx.Commit()
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// // NewSQLAdapter creates a new SQLAdapter.
// func NewSQLAdapter[C any](db *sql.DB, table string, driver string) *SQLAdapter[C any] {
// 	return &SQLAdapter[C any]{db: db, table: table, driver: driver}
// }
