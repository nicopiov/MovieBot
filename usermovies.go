package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math/rand"
	"os"
	"slices"
	"strings"
)

type UserMovies map[string][]string

func loadUserMovies(fileName string) (UserMovies, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		if _, err := os.Create(fileName); err != nil {
			return nil, fmt.Errorf("error creating file: %v", err)
		}
		return make(UserMovies), nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			slog.Error("Error closing file: " + err.Error())
		}
	}(file)

	dataFile, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var userMovies UserMovies
	if err := json.Unmarshal(dataFile, &userMovies); err != nil {
		return nil, fmt.Errorf("error unmarshalling file: %w", err)
	}
	return userMovies, nil
}

func saveUserMovies(fileName string, userMovies UserMovies) error {
	data, err := json.MarshalIndent(userMovies, "", " ")
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}
	if err := os.WriteFile(fileName, data, 0644); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}
	return nil
}

func removeMovieFromUser(userMovies UserMovies, userID string, movieToRemove string) error {
	movies, exists := userMovies[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}

	for i, movie := range movies {
		if movie == movieToRemove {
			userMovies[userID] = append(movies[:i], movies[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("movie not found in user's list")
}

func extractPersonMovies(userMovies UserMovies, firstPerson string, secondPerson string, r *rand.Rand) (string, string) {
	moviesFirst := userMovies[firstPerson]
	movie1 := moviesFirst[r.Intn(len(moviesFirst))]
	moviesSecond := userMovies[secondPerson]
	movie2 := moviesSecond[r.Intn(len(moviesSecond))]
	return movie1, movie2
}

func extractPeople(userMovies UserMovies, r *rand.Rand) (string, string) {
	tmp := maps.Keys(userMovies)
	keys := slices.Collect(tmp)
	rn := r.Intn(len(keys))
	first := keys[rn]
	keys = append(keys[:rn], keys[rn+1:]...)
	rn = r.Intn(len(keys))
	second := keys[rn]
	return first, second
}

func isMovieInUserMovies(userMovies UserMovies, movie string) bool {
	for _, movies := range userMovies {
		for _, m := range movies {
			if cleanMovieString(m) == cleanMovieString(movie) {
				return true
			}
		}
	}
	return false
}

func cleanMovieString(movie string) string {
	return strings.ToLower(strings.ReplaceAll(movie, " ", ""))
}
