package config

import (
	"os"
	"testing"

	"github.com/jesseduffield/yaml"
)

// isolateConfigDir points CONFIG_DIR at a fresh temp dir so tests never read
// or write the developer's real config folder.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CONFIG_DIR", dir)
	return dir
}

func TestDockerComposeCommandNoFiles(t *testing.T) {
	isolateConfigDir(t)
	composeFiles := []string{}
	conf, err := NewAppConfig("name", "version", "commit", "date", "buildSource", false, composeFiles, "projectDir")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actual := conf.UserConfig.CommandTemplates.DockerCompose
	expected := "docker compose"
	if actual != expected {
		t.Fatalf("Expected %s but got %s", expected, actual)
	}
}

func TestDockerComposeCommandSingleFile(t *testing.T) {
	isolateConfigDir(t)
	composeFiles := []string{"one.yml"}
	conf, err := NewAppConfig("name", "version", "commit", "date", "buildSource", false, composeFiles, "projectDir")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actual := conf.UserConfig.CommandTemplates.DockerCompose
	expected := "docker compose -f one.yml"
	if actual != expected {
		t.Fatalf("Expected %s but got %s", expected, actual)
	}
}

func TestDockerComposeCommandMultipleFiles(t *testing.T) {
	isolateConfigDir(t)
	composeFiles := []string{"one.yml", "two.yml", "three.yml"}
	conf, err := NewAppConfig("name", "version", "commit", "date", "buildSource", false, composeFiles, "projectDir")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actual := conf.UserConfig.CommandTemplates.DockerCompose
	expected := "docker compose -f one.yml -f two.yml -f three.yml"
	if actual != expected {
		t.Fatalf("Expected %s but got %s", expected, actual)
	}
}

func TestWritingToConfigFile(t *testing.T) {
	isolateConfigDir(t)
	// init the AppConfig
	emptyComposeFiles := []string{}
	conf, err := NewAppConfig("name", "version", "commit", "date", "buildSource", false, emptyComposeFiles, "projectDir")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	testFn := func(t *testing.T, ac *AppConfig, newValue bool) {
		t.Helper()
		updateFn := func(uc *UserConfig) error {
			uc.ConfirmOnQuit = newValue
			return nil
		}

		err = ac.WriteToUserConfig(updateFn)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		file, err := os.OpenFile(ac.ConfigFilename(), os.O_RDONLY, 0o660)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		sampleUC := UserConfig{}
		err = yaml.NewDecoder(file).Decode(&sampleUC)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		err = file.Close()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if sampleUC.ConfirmOnQuit != newValue {
			t.Fatalf("Got %v, Expected %v\n", sampleUC.ConfirmOnQuit, newValue)
		}
	}

	// insert value into an empty file
	testFn(t, conf, true)

	// modifying an existing file that already has 'ConfirmOnQuit'
	testFn(t, conf, false)
}
