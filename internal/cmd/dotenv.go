package cmd

import "github.com/joho/godotenv"

func loadDotenv() {
	_ = godotenv.Load()
}
