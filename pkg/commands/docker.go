package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	ogLog "log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	cliconfig "github.com/docker/cli/cli/config"
	ddocker "github.com/docker/cli/cli/context/docker"
	ctxstore "github.com/docker/cli/cli/context/store"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/imdario/mergo"
	"github.com/peauc/lazydocker-ng/pkg/commands/ssh"
	"github.com/peauc/lazydocker-ng/pkg/config"
	"github.com/peauc/lazydocker-ng/pkg/i18n"
	"github.com/peauc/lazydocker-ng/pkg/utils"
	"github.com/sasha-s/go-deadlock"
	"github.com/sirupsen/logrus"
)

const (
	dockerHostEnvKey = "DOCKER_HOST"
)

// DockerCommand is our main docker interface
type DockerCommand struct {
	Log            *logrus.Entry
	OSCommand      *OSCommand
	Tr             *i18n.TranslationSet
	Config         *config.AppConfig
	Client         *client.Client
	ErrorChan      chan error
	ContainerMutex deadlock.Mutex
	ServiceMutex   deadlock.Mutex

	Closers []io.Closer
}

var _ io.Closer = &DockerCommand{}

// LimitedDockerCommand is a stripped-down DockerCommand with just the methods the container/service/image might need
type LimitedDockerCommand interface {
	NewCommandObject(CommandObject) CommandObject
}

// CommandObject is what we pass to our template resolvers when we are running a custom command. We do not guarantee that all fields will be populated: just the ones that make sense for the current context
type CommandObject struct {
	DockerCompose string
	Service       *Service
	Container     *Container
	Image         *Image
	Volume        *Volume
	Network       *Network
}

// NewCommandObject takes a command object and returns a default command object with the passed command object merged in
func (c *DockerCommand) NewCommandObject(obj CommandObject) CommandObject {
	defaultObj := CommandObject{DockerCompose: c.Config.UserConfig.CommandTemplates.DockerCompose}
	_ = mergo.Merge(&defaultObj, obj)
	return defaultObj
}

// NewCommandObjectWithComposeFile creates a CommandObject with project-specific compose files
// Use this for commands that need the compose file (up, down, config, etc.)
func (c *DockerCommand) NewCommandObjectWithComposeFile(project *Project) CommandObject {
	dockerComposeCmd := c.Config.UserConfig.CommandTemplates.DockerCompose
	if project == nil || project.ComposeFile == "" {
		return CommandObject{DockerCompose: dockerComposeCmd}
	}

	// If the project has compose files specified in labels, add them with -f flags
	// The config_files label can contain multiple files separated by commas
	files := strings.Split(project.ComposeFile, ",")
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file != "" {
			// Extract just the filename from the absolute path
			// since we'll be running in the project's directory
			if idx := strings.LastIndex(file, "/"); idx >= 0 {
				file = file[idx+1:]
			}
			dockerComposeCmd += " -f " + file
		}
	}

	return CommandObject{DockerCompose: dockerComposeCmd}
}

// NewCommandObjectWithProjectName creates a CommandObject using the project name with -p flag
// Use this for commands that don't need the compose file (logs, ps, etc.)
// This is more reliable than using compose files since it works without the file
func (c *DockerCommand) NewCommandObjectWithProjectName(project *Project) CommandObject {
	dockerComposeCmd := c.Config.UserConfig.CommandTemplates.DockerCompose

	// Use -p flag with project name - this works without compose file
	if project != nil && project.Name != "" {
		dockerComposeCmd += " -p " + project.Name
	}

	return CommandObject{DockerCompose: dockerComposeCmd}
}

// NewDockerCommand creates a DockerCommand struct that wraps the docker client.
// Able to run docker commands and handles SSH docker hosts
func NewDockerCommand(log *logrus.Entry, osCommand *OSCommand, tr *i18n.TranslationSet, config *config.AppConfig, errorChan chan error) (*DockerCommand, error) {
	dockerHost, err := determineDockerHost()
	if err != nil {
		ogLog.Printf("> could not determine host %v", err)
	}

	tunnelResult, err := ssh.NewSSHHandler(osCommand).HandleSSHDockerHost(dockerHost)
	if err != nil {
		ogLog.Fatal(err)
	}
	// If we created a tunnel to the remote ssh host, we then override the dockerhost to point to the tunnel
	if tunnelResult.Created {
		dockerHost = tunnelResult.SocketPath
	}

	clientOpts := []client.Opt{
		client.WithTLSClientConfigFromEnv(),
		client.WithAPIVersionNegotiation(),
		client.WithHost(dockerHost),
	}

	cli, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		ogLog.Fatal(err)
	}

	dockerCommand := &DockerCommand{
		Log:       log,
		OSCommand: osCommand,
		Tr:        tr,
		Config:    config,
		Client:    cli,
		ErrorChan: errorChan,
		Closers:   []io.Closer{tunnelResult.Closer},
	}

	dockerCommand.setDockerComposeCommand(config)

	return dockerCommand, nil
}

func (c *DockerCommand) setDockerComposeCommand(config *config.AppConfig) {
	if config.UserConfig.CommandTemplates.DockerCompose != "docker compose" {
		return
	}

	// it's possible that a user is still using docker-compose, so we'll check if 'docker comopose' is available, and if not, we'll fall back to 'docker-compose'
	err := c.OSCommand.RunCommand("docker compose version")
	if err != nil {
		config.UserConfig.CommandTemplates.DockerCompose = "docker-compose"
	}
}

func (c *DockerCommand) Close() error {
	return utils.CloseMany(c.Closers)
}

func (c *DockerCommand) CreateClientStatMonitor(container *Container) {
	container.MonitoringStats = true
	stream, err := c.Client.ContainerStats(context.Background(), container.ID, true)
	if err != nil {
		// not creating error panel because if we've disconnected from docker we'll
		// have already created an error panel
		c.Log.Error(err)
		container.MonitoringStats = false
		return
	}

	defer stream.Body.Close()

	scanner := bufio.NewScanner(stream.Body)
	for scanner.Scan() {
		data := scanner.Bytes()
		var stats ContainerStats
		_ = json.Unmarshal(data, &stats)

		recordedStats := &RecordedStats{
			ClientStats: stats,
			DerivedStats: DerivedStats{
				CPUPercentage:    stats.CalculateContainerCPUPercentage(),
				MemoryPercentage: stats.CalculateContainerMemoryUsage(),
			},
			RecordedAt: time.Now(),
		}

		container.appendStats(recordedStats, c.Config.UserConfig.Stats.MaxDuration)
	}

	container.MonitoringStats = false
}

func (c *DockerCommand) RefreshContainersAndServices(currentServices []*Service, currentContainers []*Container, currentProject *Project) ([]*Container, []*Service, error) {
	c.ServiceMutex.Lock()
	defer c.ServiceMutex.Unlock()

	containers, err := c.GetContainers(currentContainers)
	if err != nil {
		return nil, nil, err
	}

	var services []*Service
	if currentProject != nil {
		services, err = c.GetServicesFromContainers(containers, currentProject)
		if err != nil {
			return nil, nil, err
		}

		c.assignContainersToServices(containers, services)
	}

	return containers, services, nil
}

func (c *DockerCommand) assignContainersToServices(containers []*Container, services []*Service) {
L:
	for _, service := range services {
		for _, ctr := range containers {
			if !ctr.OneOff && ctr.ServiceName == service.Name {
				service.Container = ctr
				continue L
			}
		}
		service.Container = nil
	}
}

// GetDockerProjects
func (c *DockerCommand) GetDockerProjects() ([]*Project, error) {
	return []*Project{}, nil
}

// GetContainers gets the docker containers
func (c *DockerCommand) GetContainers(existingContainers []*Container) ([]*Container, error) {
	c.ContainerMutex.Lock()
	defer c.ContainerMutex.Unlock()

	containers, err := c.Client.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	ownContainers := make([]*Container, len(containers))

	for i, ctr := range containers {
		var newContainer *Container

		// check if we already have data stored against the container
		for _, existingContainer := range existingContainers {
			if existingContainer.ID == ctr.ID {
				newContainer = existingContainer
				break
			}
		}

		// initialise the container if it's completely new
		if newContainer == nil {
			newContainer = &Container{
				ID:            ctr.ID,
				Client:        c.Client,
				OSCommand:     c.OSCommand,
				Log:           c.Log,
				DockerCommand: c,
				Tr:            c.Tr,
			}
		}

		newContainer.Container = ctr
		// if the container is made with a name label we will use that
		if name, ok := ctr.Labels["name"]; ok {
			newContainer.Name = name
		} else {
			if len(ctr.Names) > 0 {
				newContainer.Name = strings.TrimLeft(ctr.Names[0], "/")
			} else {
				newContainer.Name = ctr.ID
			}
		}
		newContainer.ServiceName = ctr.Labels["com.docker.compose.service"]
		newContainer.ProjectName = ctr.Labels["com.docker.compose.project"]
		newContainer.ContainerNumber = ctr.Labels["com.docker.compose.container"]
		newContainer.OneOff = ctr.Labels["com.docker.compose.oneoff"] == "True"

		ownContainers[i] = newContainer
	}

	c.SetContainerDetails(ownContainers)

	return ownContainers, nil
}

// GetServicesFromContainers gets services
func (c *DockerCommand) GetServicesFromContainers(containers []*Container, currentProject *Project) ([]*Service, error) {
	services := make([]*Service, 0, len(containers))

	if currentProject.Name == "" {
		return services, nil
	}

	for _, cont := range containers {
		if cont.ProjectName != currentProject.Name {
			continue
		}

		service := &Service{
			Name:          cont.ServiceName,
			ID:            cont.ID,
			OSCommand:     c.OSCommand,
			Log:           c.Log,
			DockerCommand: c,
		}
		services = append(services, service)
	}

	// TODO: Should be run once every time the project changes
	if len(services) == 0 {
		// If no services are found in running containers fetch services from dockerCompose
		var err error
		services, err = c.GetServices(currentProject != nil && currentProject.IsDockerCompose)
		if err != nil {
			return nil, err
		}
	}

	return services, nil
}

// GetServices gets services
func (c *DockerCommand) GetServices(inComposeProject bool) ([]*Service, error) {
	if !inComposeProject {
		return nil, nil
	}

	composeCommand := c.Config.UserConfig.CommandTemplates.DockerCompose
	// TODO: Handle remote docker compose files
	output, err := c.OSCommand.RunCommandWithOutput(fmt.Sprintf("%s config --services", composeCommand))
	if err != nil {
		return nil, err
	}

	// output looks like:
	// service1
	// service2
	lines := utils.SplitLines(output)
	services := make([]*Service, len(lines))
	for i, str := range lines {
		services[i] = &Service{
			Name:          str,
			ID:            str,
			OSCommand:     c.OSCommand,
			Log:           c.Log,
			DockerCommand: c,
		}
	}

	return services, nil
}

func (c *DockerCommand) RefreshContainerDetails(containers []*Container) error {
	c.ContainerMutex.Lock()
	defer c.ContainerMutex.Unlock()

	c.SetContainerDetails(containers)

	return nil
}

// SetContainerDetails Attaches the details returned from docker inspect to each of the containers
// this contains a bit more info than what you get from the go-docker client
func (c *DockerCommand) SetContainerDetails(containers []*Container) {
	wg := sync.WaitGroup{}
	for _, ctr := range containers {
		ctr := ctr
		wg.Add(1)
		go func() {
			details, err := c.Client.ContainerInspect(context.Background(), ctr.ID)
			if err != nil {
				c.Log.Error(err)
			} else {
				ctr.Details = details
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// ViewAllLogs attaches to a subprocess viewing all the logs from docker-compose
func (c *DockerCommand) ViewAllLogs() (*exec.Cmd, error) {
	cmd := c.OSCommand.ExecutableFromString(
		utils.ApplyTemplate(
			c.OSCommand.Config.UserConfig.CommandTemplates.ViewAllLogs,
			c.NewCommandObject(CommandObject{}),
		),
	)

	c.OSCommand.PrepareForChildren(cmd)

	return cmd, nil
}

// DockerComposeConfig returns the result of 'docker-compose config'
func (c *DockerCommand) DockerComposeConfig() string {
	output, err := c.OSCommand.RunCommandWithOutput(
		utils.ApplyTemplate(
			c.OSCommand.Config.UserConfig.CommandTemplates.DockerComposeConfig,
			c.NewCommandObject(CommandObject{}),
		),
	)
	if err != nil {
		output = err.Error()
	}
	return output
}

// DockerComposeConfigForProject gets the docker compose config for a specific project
func (c *DockerCommand) DockerComposeConfigForProject(projectPath string) string {
	output, _ := c.DockerComposeConfigForProjectWithError(projectPath)
	return output
}

// DockerComposeConfigForProjectWithError gets the docker compose config and returns the error separately
func (c *DockerCommand) DockerComposeConfigForProjectWithError(projectPath string) (string, error) {
	if projectPath == "" {
		output, err := c.OSCommand.RunCommandWithOutput(
			utils.ApplyTemplate(
				c.OSCommand.Config.UserConfig.CommandTemplates.DockerComposeConfig,
				c.NewCommandObject(CommandObject{}),
			),
		)
		return output, err
	}

	cmd := c.OSCommand.ExecutableFromString(
		utils.ApplyTemplate(
			c.OSCommand.Config.UserConfig.CommandTemplates.DockerComposeConfig,
			c.NewCommandObject(CommandObject{}),
		),
	)
	cmd.Dir = projectPath

	output, err := c.OSCommand.RunExecutableWithOutput(cmd)
	return output, err
}

// IsDockerComposeFileNotFoundError checks if the error is due to missing docker-compose file
func IsDockerComposeFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no configuration file provided") ||
		strings.Contains(errStr, "not found")
}

// IsDockerComposeYAMLError checks if the error is due to YAML parsing issues
func IsDockerComposeYAMLError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "yaml:") ||
		strings.Contains(errStr, "parsing")
}

// IsDockerComposeValidationError checks if the error is due to schema validation
func IsDockerComposeValidationError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "error decoding") ||
		strings.Contains(errStr, "validation failed") ||
		strings.Contains(errStr, "invalid")
}

// GetProjectServiceCount returns the number of services defined in a project's docker-compose file
func (c *DockerCommand) GetProjectServiceCount(projectPath string) (int, error) {
	composeCommand := c.Config.UserConfig.CommandTemplates.DockerCompose
	cmdStr := fmt.Sprintf("%s config --services", composeCommand)

	if projectPath == "" {
		output, err := c.OSCommand.RunCommandWithOutput(cmdStr)
		if err != nil {
			return -1, err
		}
		lines := utils.SplitLines(output)
		return len(lines), nil
	}

	cmd := c.OSCommand.ExecutableFromString(cmdStr)
	cmd.Dir = projectPath
	output, err := c.OSCommand.RunExecutableWithOutput(cmd)
	if err != nil {
		return -1, err
	}

	lines := utils.SplitLines(output)
	return len(lines), nil
}

// GetProjects extracts project information from containers and optionally adds the current directory project
func (c *DockerCommand) GetProjects(containers []*Container, currentProjectDir string, startedInComposeDir bool) []*Project {
	projectsMap := make(map[string]*Project)
	servicesPerProject := make(map[string]map[string]bool)

	// Build project info from containers
	for _, container := range containers {
		projectName, exists := container.Container.Labels["com.docker.compose.project"]
		if !exists || projectName == "" {
			continue
		}

		if _, ok := projectsMap[projectName]; !ok {
			projectsMap[projectName] = &Project{
				Name:            projectName,
				IsDockerCompose: true,
				Status:          "unknown",
				LastUpdated:     time.Now(),
			}
			servicesPerProject[projectName] = make(map[string]bool)
		}

		project := projectsMap[projectName]
		project.ContainerCount++

		if container.Container.State == "running" {
			project.RunningCount++
		}

		if project.Path == "" {
			if workingDir, ok := container.Container.Labels["com.docker.compose.project.working_dir"]; ok {
				project.Path = workingDir
			}
		}

		if project.ComposeFile == "" {
			if configFiles, ok := container.Container.Labels["com.docker.compose.project.config_files"]; ok {
				project.ComposeFile = configFiles
			}
		}

		if serviceName, ok := container.Container.Labels["com.docker.compose.service"]; ok && serviceName != "" {
			servicesPerProject[projectName][serviceName] = true
		}
	}

	projectsList := make([]*Project, 0, len(projectsMap))
	for _, project := range projectsMap {
		projectsList = append(projectsList, project)
	}

	// Add current directory project if we started in a compose directory and it's not already in the list
	if startedInComposeDir && currentProjectDir != "" {
		currentDirProjectName := strings.TrimPrefix(currentProjectDir, "/")
		if idx := strings.LastIndex(currentDirProjectName, "/"); idx >= 0 {
			currentDirProjectName = currentDirProjectName[idx+1:]
		}

		if _, exists := projectsMap[currentDirProjectName]; !exists {
			currentDirProject := &Project{
				Name:            currentDirProjectName,
				Path:            currentProjectDir,
				IsDockerCompose: true,
				Status:          "not created",
			}

			// Get service count from docker-compose.yml
			serviceCount, err := c.GetProjectServiceCount(currentProjectDir)
			if err == nil {
				currentDirProject.ServiceCount = serviceCount
			} else {
				currentDirProject.ServiceCount = -1
			}

			projectsList = append(projectsList, currentDirProject)
		}
	}

	// Calculate status and service counts for all projects
	for _, project := range projectsMap {
		switch project.RunningCount {
		case 0:
			project.Status = "stopped"
		case project.ContainerCount:
			project.Status = "running"
		default:
			project.Status = "mixed"
		}

		project.ServiceCount = len(servicesPerProject[project.Name])

		// If we have a path and no service count, try to get it from docker-compose.yml
		if project.ServiceCount == 0 && project.Path != "" {
			serviceCount, err := c.GetProjectServiceCount(project.Path)
			if err == nil {
				project.ServiceCount = serviceCount
			}
		}
	}

	return projectsList
}

// determineDockerHost tries to the determine the docker host that we should connect to
// in the following order of decreasing precedence:
//   - value of "DOCKER_HOST" environment variable
//   - host retrieved from the current context (specified via DOCKER_CONTEXT)
//   - "default docker host" for the host operating system, otherwise
func determineDockerHost() (string, error) {
	// If the docker host is explicitly set via the "DOCKER_HOST" environment variable,
	// then its a no-brainer :shrug:
	if os.Getenv("DOCKER_HOST") != "" {
		return os.Getenv("DOCKER_HOST"), nil
	}

	currentContext := os.Getenv("DOCKER_CONTEXT")
	if currentContext == "" {
		cf, err := cliconfig.Load(cliconfig.Dir())
		if err != nil {
			return "", err
		}
		currentContext = cf.CurrentContext
	}

	// On some systems (windows) `default` is stored in the docker config as the currentContext.
	if currentContext == "" || currentContext == "default" {
		// If a docker context is neither specified via the "DOCKER_CONTEXT" environment variable nor via the
		// $HOME/.docker/config file, then we fall back to connecting to the "default docker host" meant for
		// the host operating system.
		return defaultDockerHost, nil
	}

	storeConfig := ctxstore.NewConfig(
		func() interface{} { return &ddocker.EndpointMeta{} },
		ctxstore.EndpointTypeGetter(ddocker.DockerEndpoint, func() interface{} { return &ddocker.EndpointMeta{} }),
	)

	st := ctxstore.New(cliconfig.ContextStoreDir(), storeConfig)
	md, err := st.GetMetadata(currentContext)
	if err != nil {
		return "", err
	}
	dockerEP, ok := md.Endpoints[ddocker.DockerEndpoint]
	if !ok {
		return "", err
	}
	dockerEPMeta, ok := dockerEP.(ddocker.EndpointMeta)
	if !ok {
		return "", fmt.Errorf("expected docker.EndpointMeta, got %T", dockerEP)
	}

	if dockerEPMeta.Host != "" {
		return dockerEPMeta.Host, nil
	}

	// We might end up here, if the context was created with the `host` set to an empty value (i.e. '').
	// For example:
	// ```sh
	// docker context create foo --docker "host="
	// ```
	// In such scenario, we mimic the `docker` cli and try to connect to the "default docker host".
	return defaultDockerHost, nil
}
