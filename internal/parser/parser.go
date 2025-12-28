package parser

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/strrl/auto-flavor/internal/db"
)

type Parser struct {
	db       *sql.DB
	claudeDir string
}

func NewParser() (*Parser, error) {
	database, err := db.GetDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	return &Parser{
		db:       database,
		claudeDir: filepath.Join(homeDir, ".claude", "projects"),
	}, nil
}

func (p *Parser) FetchEntriesForProject(projectPath string, since time.Time) ([]*ParsedEntry, error) {
	var query string
	var rows *sql.Rows
	var err error

	if since.IsZero() {
		query = fmt.Sprintf(`
			SELECT
				type,
				CAST(to_json(message) AS VARCHAR) as message_json,
				CAST(timestamp AS VARCHAR) as ts,
				CAST(sessionId AS VARCHAR) as session_id,
				CAST(uuid AS VARCHAR) as uuid,
				COALESCE(CAST(parentUuid AS VARCHAR), '') as parent_uuid,
				COALESCE(cwd, '') as cwd
			FROM read_json('%s/**/*.jsonl',
				format = 'newline_delimited',
				union_by_name = true,
				ignore_errors = true
			)
			WHERE cwd = $1
			  AND type IN ('user', 'assistant')
			  AND message IS NOT NULL
			ORDER BY timestamp ASC
		`, p.claudeDir)
		rows, err = p.db.Query(query, projectPath)
	} else {
		sinceStr := since.Format("2006-01-02T15:04:05")
		query = fmt.Sprintf(`
			SELECT
				type,
				CAST(to_json(message) AS VARCHAR) as message_json,
				CAST(timestamp AS VARCHAR) as ts,
				CAST(sessionId AS VARCHAR) as session_id,
				CAST(uuid AS VARCHAR) as uuid,
				COALESCE(CAST(parentUuid AS VARCHAR), '') as parent_uuid,
				COALESCE(cwd, '') as cwd
			FROM read_json('%s/**/*.jsonl',
				format = 'newline_delimited',
				union_by_name = true,
				ignore_errors = true
			)
			WHERE cwd = $1
			  AND timestamp >= $2
			  AND type IN ('user', 'assistant')
			  AND message IS NOT NULL
			ORDER BY timestamp ASC
		`, p.claudeDir)
		rows, err = p.db.Query(query, projectPath, sinceStr)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []*ParsedEntry
	for rows.Next() {
		var (
			entryType   string
			messageJSON string
			timestamp   string
			sessionID   string
			uuid        string
			parentUUID  string
			cwd         string
		)

		if err := rows.Scan(&entryType, &messageJSON, &timestamp, &sessionID, &uuid, &parentUUID, &cwd); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339, timestamp)

		entry := &ChatEntry{
			Type:       entryType,
			Message:    json.RawMessage(messageJSON),
			Timestamp:  ts,
			SessionID:  sessionID,
			UUID:       uuid,
			ParentUUID: parentUUID,
			CWD:        cwd,
		}

		parsed, err := entry.Parse()
		if err != nil {
			continue
		}

		entries = append(entries, parsed)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return entries, nil
}

func (p *Parser) ListProjects() ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT cwd
		FROM read_json('%s/**/*.jsonl',
			format = 'newline_delimited',
			union_by_name = true,
			ignore_errors = true
		)
		WHERE cwd IS NOT NULL AND cwd != ''
		ORDER BY cwd
	`, p.claudeDir)

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var cwd string
		if err := rows.Scan(&cwd); err != nil {
			continue
		}
		projects = append(projects, cwd)
	}

	return projects, nil
}

func (p *Parser) GetProjectStats(projectPath string) (int, time.Time, time.Time, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as count,
			MIN(timestamp) as first,
			MAX(timestamp) as last
		FROM read_json('%s/**/*.jsonl',
			format = 'newline_delimited',
			union_by_name = true,
			ignore_errors = true
		)
		WHERE cwd = $1
		  AND type IN ('user', 'assistant')
	`, p.claudeDir)

	var count int
	var firstStr, lastStr sql.NullString

	err := p.db.QueryRow(query, projectPath).Scan(&count, &firstStr, &lastStr)
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("failed to get stats: %w", err)
	}

	var first, last time.Time
	if firstStr.Valid {
		first, _ = time.Parse(time.RFC3339, firstStr.String)
	}
	if lastStr.Valid {
		last, _ = time.Parse(time.RFC3339, lastStr.String)
	}

	return count, first, last, nil
}
