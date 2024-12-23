package ingestor

import (
	"owlrepo/utils"
	"testing"
)

func TestIngestor(t *testing.T) {
	utils.LoadEnv("../back.env", true)
	Ingest()
}
