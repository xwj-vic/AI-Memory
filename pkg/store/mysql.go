package store

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/types"
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// NewMySQLStore initializes a new MySQL connection pool.
func NewMySQLStore(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBHost,
		cfg.DBName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	return db, nil
}

// MySQLEndUserStore implements memory.EndUserStore.
type MySQLEndUserStore struct {
	db *sql.DB
}

func NewMySQLEndUserStore(db *sql.DB) *MySQLEndUserStore {
	return &MySQLEndUserStore{db: db}
}

func (s *MySQLEndUserStore) Init() error {
	query := `
		CREATE TABLE IF NOT EXISTS end_users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_identifier VARCHAR(255) NOT NULL UNIQUE,
			last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := s.db.Exec(query)
	return err
}

func (s *MySQLEndUserStore) UpsertUser(ctx context.Context, identifier string) error {
	query := `
		INSERT INTO end_users (user_identifier, last_active) 
		VALUES (?, NOW()) 
		ON DUPLICATE KEY UPDATE last_active = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, identifier)
	return err
}

func (s *MySQLEndUserStore) ListUsers(ctx context.Context) ([]types.EndUser, error) {
	query := `SELECT id, user_identifier, last_active, created_at FROM end_users ORDER BY last_active DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []types.EndUser
	for rows.Next() {
		var u types.EndUser
		if err := rows.Scan(&u.ID, &u.UserIdentifier, &u.LastActive, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
