package ingestor

import (
	"owlrepo/utils"
	"testing"
)

func TestIngestFromOwlRepo(t *testing.T) {
	utils.LoadEnv("../back.env", true)
	Ingest()
}
