package gherkin_test

import (
	"embed"
	"testing"

	"github.com/cucumber/godog"
)

//go:embed features/*.feature
var features embed.FS

func TestMain(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:         "pretty",
			TestingT:       t, // Testing instance that will run subtests.
			FS:             features,
			DefaultContext: t.Context(),
		},
	}

	if exit := suite.Run(); exit != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests", exit)
	}
}
