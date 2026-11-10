package commands

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/peauc/lazydocker-ng/pkg/config"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func newComposeTestCommand() *DockerCommand {
	return &DockerCommand{Config: &config.AppConfig{UserConfig: &config.UserConfig{
		CommandTemplates: config.CommandTemplatesConfig{DockerCompose: "docker compose"},
	}}}
}

func TestComposeFileArgsSurviveSpaces(t *testing.T) {
	project := &Project{ComposeFile: "/home/me/My Project/docker compose.yml,/x/override.yml"}

	obj := newComposeTestCommand().NewCommandObjectWithComposeFile(project)

	args, err := utils.SplitArgs(obj.DockerCompose)
	assert.NoError(t, err)
	assert.Equal(t, []string{"docker", "compose", "-f", "docker compose.yml", "-f", "override.yml"}, args)
}

func TestProjectNameIsOneArg(t *testing.T) {
	project := &Project{Name: "x -f evil.yml"}

	obj := newComposeTestCommand().NewCommandObjectWithProjectName(project)

	args, err := utils.SplitArgs(utils.ApplyTemplate("{{ .DockerCompose }} ps", obj))
	assert.NoError(t, err)
	assert.Equal(t, []string{"docker", "compose", "-p", "x -f evil.yml", "ps"}, args)
}

func TestComposeLabels(t *testing.T) {
	for value, want := range map[string]string{
		"myproj_1": "myproj_1", "a-b": "a-b", "0proj": "0proj",
		"x -f evil.yml": "", "-x": "", "Upper": "", "": "", "_x": "",
	} {
		project, _ := composeLabels(map[string]string{"com.docker.compose.project": value})
		assert.Equal(t, want, project, "project label %q", value)
	}
	for value, want := range map[string]string{
		"web": "web", ".hidden": ".hidden", "svc_2": "svc_2", "Web-1": "Web-1",
		"--timeout=0": "", "a b": "", "-x": "", "": "",
	} {
		_, service := composeLabels(map[string]string{"com.docker.compose.service": value})
		assert.Equal(t, want, service, "service label %q", value)
	}
}

func TestGetProjectsSkipsInvalidLabels(t *testing.T) {
	containers := []*Container{
		{Container: container.Summary{Labels: map[string]string{
			"com.docker.compose.project": "x -f evil", "com.docker.compose.service": "web",
		}}},
		{Container: container.Summary{Labels: map[string]string{
			"com.docker.compose.project": "good", "com.docker.compose.service": "web",
		}}},
	}

	projects := newComposeTestCommand().GetProjects(containers, "", false)

	names := make([]string, 0, len(projects))
	for _, project := range projects {
		names = append(names, project.Name)
	}
	assert.Equal(t, []string{"good"}, names)
}
