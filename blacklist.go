package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

type Blacklist []string

func loadBlacklist(filename string) (Blacklist, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		if _, err := os.Create(filename); err != nil {
			return nil, fmt.Errorf("error creating file: %w", err)
		}
		return make(Blacklist, 0), nil
	}
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return Blacklist{}, nil
		}
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			slog.Error("Error closing file: " + err.Error())
		}
	}(file)

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var blacklist Blacklist
	if len(data) > 0 {
		if err := json.Unmarshal(data, &blacklist); err != nil {
			return nil, fmt.Errorf("error unmarshalling file: %w", err)
		}
	}
	return blacklist, nil
}

func saveBlacklist(filename string, blacklist Blacklist) error {
	data, err := json.MarshalIndent(blacklist, "", " ")
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}

func updateBlacklist(filename string, entry string) error {
	blacklist, err := loadBlacklist(filename)
	if err != nil {
		return fmt.Errorf("error loading blacklist file: %w", err)
	}
	blacklist = append(blacklist, entry)
	if err := saveBlacklist(blacklistFile, blacklist); err != nil {
		return fmt.Errorf("error saving blacklist file: %w", err)
	}
	return nil
}
