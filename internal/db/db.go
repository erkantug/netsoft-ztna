package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type WizardDB struct {
	db *sql.DB
}

func New(path string) (*WizardDB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	w := &WizardDB{db: db}
	if err := w.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return w, nil
}

func (w *WizardDB) Close() error {
	return w.db.Close()
}

func (w *WizardDB) migrate() error {
	_, err := w.db.Exec(`
		CREATE TABLE IF NOT EXISTS wizard_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			current_step INTEGER NOT NULL DEFAULT 1,
			data_json TEXT NOT NULL DEFAULT '{}',
			deployed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT OR IGNORE INTO wizard_state (id, current_step) VALUES (1, 1);
	`)
	return err
}

func (w *WizardDB) GetCurrentStep() (Step, error) {
	var s int
	err := w.db.QueryRow("SELECT current_step FROM wizard_state WHERE id = 1").Scan(&s)
	return Step(s), err
}

func (w *WizardDB) SetCurrentStep(s Step) error {
	_, err := w.db.Exec("UPDATE wizard_state SET current_step = ?, updated_at = ? WHERE id = 1",
		int(s), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (w *WizardDB) SaveStepData(step Step, data interface{}) error {
	// First get existing data
	all, err := w.GetAllData()
	if err != nil {
		return err
	}
	// Update the specific step
	switch step {
	case StepNetwork:
		if d, ok := data.(*NetworkData); ok {
			all.Network = d
		}
	case StepTLS:
		if d, ok := data.(*TLSData); ok {
			all.TLS = d
		}
	case StepAdmin:
		if d, ok := data.(*AdminData); ok {
			all.Admin = d
		}
	case StepIDP:
		if d, ok := data.(*IDPData); ok {
			all.IDP = d
		}
	}
	b, err := json.Marshal(all)
	if err != nil {
		return err
	}
	_, err = w.db.Exec("UPDATE wizard_state SET data_json = ?, current_step = ?, updated_at = ? WHERE id = 1",
		string(b), int(step)+1, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (w *WizardDB) GetAllData() (*AllData, error) {
	var data string
	err := w.db.QueryRow("SELECT data_json FROM wizard_state WHERE id = 1").Scan(&data)
	if err != nil {
		return nil, err
	}
	all := &AllData{}
	if err := json.Unmarshal([]byte(data), all); err != nil {
		return nil, err
	}
	return all, nil
}

func (w *WizardDB) MarkDeployed() error {
	_, err := w.db.Exec("UPDATE wizard_state SET deployed = 1, updated_at = ? WHERE id = 1",
		time.Now().UTC().Format(time.RFC3339))
	return err
}

func (w *WizardDB) IsDeployed() (bool, error) {
	var d int
	err := w.db.QueryRow("SELECT deployed FROM wizard_state WHERE id = 1").Scan(&d)
	return d == 1, err
}
