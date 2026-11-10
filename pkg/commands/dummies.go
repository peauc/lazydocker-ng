package commands

import (
	"io"

	"github.com/peauc/lazydocker-ng/pkg/config"
	"github.com/sirupsen/logrus"
)

// This file exports dummy constructors for use by tests in other packages

// NewDummyOSCommand creates a new dummy OSCommand for testing
func NewDummyOSCommand() *OSCommand {
	return NewOSCommand(NewDummyLog(), NewDummyAppConfig())
}

// NewDummyAppConfig creates a new dummy AppConfig for testing
func NewDummyAppConfig() *config.AppConfig {
	appConfig := &config.AppConfig{
		Name:        "lazydocker",
		Version:     "unversioned",
		Commit:      "",
		BuildDate:   "",
		Debug:       false,
		BuildSource: "",
	}
	return appConfig
}

// NewDummyLog creates a new dummy Log for testing
func NewDummyLog() *logrus.Entry {
	log := logrus.New()
	log.Out = io.Discard
	return log.WithField("test", "test")
}
