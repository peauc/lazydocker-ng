# User Config:

## Opening The User Config

The location of the user config will differ depending on your OS. You can open it via lazydocker by opening the application, clicking on the 'project' panel at the top left and pressing 'o' for graphical edit or pressing 'e' for terminal edit.

Changes to the user config will only take place after closing and re-opening lazydocker

### Locations:

- macOS: `~/Library/Application Support/lazydocker-ng/config.yml`
- Linux: `~/.config/lazydocker-ng/config.yml`
- Windows: `%APPDATA%\lazydocker-ng\config.yml`

If `XDG_CONFIG_HOME` is set, the file is `$XDG_CONFIG_HOME/lazydocker-ng/config.yml` on every OS. `CONFIG_DIR` replaces the folder.

**Coming from lazydocker:** if lazydocker-ng has no `config.yml` yet (or only the empty one it creates on a first run), it reads lazydocker's (`jesseduffield/lazydocker/config.yml`, then `lazydocker/config.yml`, in the same config folder) and never changes it. The first time you edit the config from lazydocker-ng (`e` or `o` in the project panel), it saves a copy in its own folder and uses that from then on.

## Default:

```yml
gui:
  scrollHeight: 2
  language: "auto" # one of 'auto' | 'en' | 'de' | 'es' | 'fr' | 'nl' | 'pl' | 'pt' | 'tr' | 'zh'
  border: "rounded" # one of 'rounded' | 'single' | 'double' | 'hidden'
  theme:
    activeBorderColor:
      - green
      - bold
    inactiveBorderColor:
      - white
    selectedLineBgColor:
      - blue
    optionsTextColor:
      - blue
  returnImmediately: false
  wrapMainPanel: true
  # Side panel width as a ratio of the screen's width
  sidePanelWidth: 0.333
  # Determines whether we show the bottom line (the one containing keybinding
  # info and the status of the app).
  showBottomLine: true
  # When true, increases vertical space used by focused side panel,
  # creating an accordion effect
  expandFocusedSidePanel: false
  # Determines which screen mode will be used on startup
  screenMode: "normal" # one of 'normal' | 'half' | 'fullscreen'
  # Determines the style of the container status and container health display in the
  # containers panel. "long": full words (default), "short": one or two characters,
  # "icon": unicode emoji.
  containerStatusHealthStyle: "long"
logs:
  timestamps: false
  since: '60m' # set to '' to show all logs
  tail: '' # set to 200 to show last 200 lines of logs
commandTemplates:
  dockerCompose: docker compose # Determines the Docker Compose command to run, referred to as .DockerCompose in commandTemplates
  restartService: '{{ .DockerCompose }} restart {{ .Service.Name }}'
  up:  '{{ .DockerCompose }} up -d'
  down: '{{ .DockerCompose }} down'
  downWithVolumes: '{{ .DockerCompose }} down --volumes'
  upService:  '{{ .DockerCompose }} up -d {{ .Service.Name }}'
  startService: '{{ .DockerCompose }} start {{ .Service.Name }}'
  stopService: '{{ .DockerCompose }} stop {{ .Service.Name }}'
  serviceLogs: '{{ .DockerCompose }} logs --since=60m --follow {{ .Service.Name }}'
  viewServiceLogs: '{{ .DockerCompose }} logs --follow {{ .Service.Name }}'
  rebuildService: '{{ .DockerCompose }} up -d --build {{ .Service.Name }}'
  recreateService: '{{ .DockerCompose }} up -d --force-recreate {{ .Service.Name }}'
  allLogs: '{{ .DockerCompose }} logs --tail=300 --follow'
  viewAlLogs: '{{ .DockerCompose }} logs'
  dockerComposeConfig: '{{ .DockerCompose }} config'
  checkDockerComposeConfig: '{{ .DockerCompose }} config --quiet'
  serviceTop: '{{ .DockerCompose }} top {{ .Service.Name }}'
oS:
  openCommand: open {{filename}}
  openLinkCommand: open {{link}}
stats:
  graphs:
    - caption: CPU (%)
      statPath: DerivedStats.CPUPercentage
      color: blue
    - caption: Memory (%)
      statPath: DerivedStats.MemoryPercentage
      color: green
dockerApiVersion: ''
```

## To see what all of the config options mean, and what other options you can set, see [here](https://pkg.go.dev/github.com/peauc/lazydocker-ng/pkg/config)

## Color Attributes:

For color attributes you can choose an array of attributes (with max one color attribute)
The available attributes are:

- default
- black
- red
- green
- yellow
- blue
- magenta
- cyan
- white
- bold
- reverse # useful for high-contrast
- underline

## Custom Commands

You can add custom commands like so:

```yaml
customCommands:
  containers:
    - name: bash
      attach: true
      command: 'docker exec -it {{ .Container.ID }} bash'
      serviceNames: []
```

You may use the following go templates (such as `{{ .Container.ID }}` above) in your commands:
- `{{ .DockerCompose }}`: the docker compose command (default: `docker compose`)
- [`{{ .Container }}`](https://pkg.go.dev/github.com/peauc/lazydocker-ng/pkg/commands#Container) and its fields. For example: `{{ .Container.Container.ImageID }}`
- [`{{ .Service }}`](https://pkg.go.dev/github.com/peauc/lazydocker-ng/pkg/commands#Service) and its fields. For example: `{{ .Service.Name }}`

## Replacements

You can add replacements like so:

```yaml
replacements:
  imageNamePrefixes:
    '123456789012.dkr.ecr.us-east-1.amazonaws.com': '<prod>'
    '923456789999.dkr.ecr.us-east-1.amazonaws.com': '<dev>'
```
