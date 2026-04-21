package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"

	"github.com/google/uuid"
)

const dbProvider = "sqlite3"

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS profiles (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL UNIQUE,
			gender              TEXT NOT NULL,
			gender_probability  REAL NOT NULL,
			age                 INTEGER NOT NULL,
			age_group           TEXT NOT NULL,
			country_id          TEXT NOT NULL,
			country_name        TEXT NOT NULL,
			country_probability REAL NOT NULL,
			created_at          TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_name ON profiles (LOWER(name));
	`)
	return err
}

func Seed(db *sql.DB, seedPath string) error {
	f, err := os.Open(seedPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var data struct {
		Profiles []struct {
			Name               string  `json:"name"`
			Gender             string  `json:"gender"`
			GenderProbability  float64 `json:"gender_probability"`
			Age                int     `json:"age"`
			AgeGroup           string  `json:"age_group"`
			CountryID          string  `json:"country_id"`
			CountryName        string  `json:"country_name"`
			CountryProbability float64 `json:"country_probability"`
		} `json:"profiles"`
	}
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return err
	}

	stmt, err := db.Prepare(`
		INSERT OR IGNORE INTO profiles
			(id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range data.Profiles {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(id.String(), p.Name, p.Gender, p.GenderProbability, p.Age, p.AgeGroup, p.CountryID, p.CountryName, p.CountryProbability); err != nil {
			return err
		}
	}
	return nil
}

func BootstrapDB(connString string, seedPath string) (*sql.DB, error) {
	log.Println("Opening connection to db")
	db, err := sql.Open(dbProvider, connString)
	if err != nil {
		log.Println("Failed to open db:", err)
		return nil, err
	}

	log.Println("Running migrations")
	err = Migrate(db)
	if err != nil {
		log.Println("Failed to migrate db:", err)
		return nil, err
	}

	log.Println("Initialising db with seed data")
	err = Seed(db, seedPath)

	if err != nil {
		log.Println("Initialising db with seed data failed:", err)
		return nil, err
	}

	return db, nil
}
