package server

import (
	"database/sql"
	"encoding/json"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type store struct{ db *sql.DB }

func openStore(dir string) (*store, error) {
	db, err := sql.Open("sqlite", filepath.Join(dir, "db.sipc"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", "CREATE TABLE IF NOT EXISTS go_uploads (id TEXT PRIMARY KEY, data TEXT NOT NULL)", "CREATE TABLE IF NOT EXISTS go_jobs (id TEXT PRIMARY KEY, data TEXT NOT NULL)", "CREATE TABLE IF NOT EXISTS go_job_files (job_id TEXT NOT NULL, sequence INTEGER NOT NULL, data TEXT NOT NULL, PRIMARY KEY(job_id,sequence))"} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &store{db}, nil
}

func (s *store) saveJob(state JobState) error {
	log := state.Log
	state.Log = nil
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO go_jobs(id,data) VALUES (?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", state.ID, string(b)); err != nil {
		return err
	}
	if len(log) > 0 {
		log = log[len(log)-1:]
	}
	for _, entry := range log {
		b, err = json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO go_job_files(job_id,sequence,data) VALUES (?,?,?) ON CONFLICT(job_id,sequence) DO UPDATE SET data=excluded.data", state.ID, entry.Sequence, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *store) loadJobLog(id string) ([]FileLogEntry, error) {
	rows, err := s.db.Query("SELECT data FROM go_job_files WHERE job_id=? ORDER BY sequence", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []FileLogEntry
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var e FileLogEntry
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
func (s *store) save(table, id string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO "+table+" (id,data) VALUES (?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", id, string(b))
	return err
}
func (s *store) load(table string) ([][]byte, error) {
	rows, err := s.db.Query("SELECT data FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result [][]byte
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
