package utils

import (
	"fmt"

	"github.com/joho/godotenv"
)

func LoadEnv(filePath string, isCritical bool) error {
	if filePath == "" {
		filePath = ".env"
	}

	err := godotenv.Load(filePath)
	if err != nil && isCritical {
		panic(fmt.Sprintf("Error loading .env file %s", filePath))
	} else if err != nil {
		return fmt.Errorf("Error loading .env file %s", filePath)
	}

	return nil
}
