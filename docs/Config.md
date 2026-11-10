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
  theme:
    activeBorderColor:
      - green
      - bold
    inactiveBorderColor:
      - default
    selectedLineBgColor:
      - blue
    optionsTextColor:
      - blue
  returnImmediately: false
  wrapMainPanel: true
  # Side panel width as a ratio of the screen's width
  sidePanelWidth: 0.3333
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
  border: "" # '' or 'rounded' (default) | 'single' | 'double' | 'hidden'
  # true turns mouse support off (configs written for lazydocker may use mouseEvents)
  ignoreMouseEvents: false
logs:
  timestamps: false
  since: '60m' # set to '' to show all logs
  tail: '' # set to 200 to show last 200 lines of logs
commandTemplates:
  dockerCompose: docker compose # Determines the Docker Compose command to run, referred to as .DockerCompose in commandTemplates
  restartService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} restart {{ .Service.Name }}'
  startService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} start {{ .Service.Name }}'
  upService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} up -d {{ .Service.Name }}'
  up: '{{ .DockerCompose }} up -d'
  down: '{{ .DockerCompose }} down'
  downWithVolumes: '{{ .DockerCompose }} down --volumes'
  start: '{{ .DockerCompose }} start'
  stop: '{{ .DockerCompose }} stop'
  stopService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} stop {{ .Service.Name }}'
  serviceLogs: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} logs --since=60m --follow {{ .Service.Name }}'
  viewServiceLogs: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} logs --follow {{ .Service.Name }}'
  rebuildService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} up -d --build {{ .Service.Name }}'
  recreateService: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} up -d --force-recreate {{ .Service.Name }}'
  allLogs: '{{ .DockerCompose }} logs --tail=300 --follow'
  viewAllLogs: '{{ .DockerCompose }} logs'
  dockerComposeConfig: '{{ .DockerCompose }} config'
  checkDockerComposeConfig: '{{ .DockerCompose }} config --quiet'
  serviceTop: '{{ .DockerCompose }} --project-name {{ .Service.ProjectName }} top {{ .Service.Name }}'
  preferredExecShell: [] # shells tried in order when attaching to a container, e.g. [zsh, bash]
# macOS values; Linux uses `sh -c "xdg-open {{filename}} >/dev/null"` and Windows `cmd /c "start "" {{filename}}"`
os:
  openCommand: open {{filename}}
  openLinkCommand: open {{link}}
stats:
  maxDuration: 3m0s
  graphs:
    - caption: CPU (%)
      statPath: DerivedStats.CPUPercentage
      color: cyan
    - caption: Memory (%)
      statPath: DerivedStats.MemoryPercentage
      color: green
dockerApiVersion: '' # e.g. '1.44' to use that API version instead of negotiating one
```

The defaults also include bulk commands for services (up, stop, pull, build, down…). Run `lazydocker-ng --config` to print every default.

## Renamed keys

| Old key (still works) | New key |
|---|---|
| `oS` | `os` |
| `gui.mouseEvents` | `gui.ignoreMouseEvents` |
| `commandTemplates.viewAlLogs` | `commandTemplates.viewAllLogs` |
| `commandTemplates.preferedExecShell` | `commandTemplates.preferredExecShell` |

The old spellings still work. If both are present, the new one wins. Keys that lazydocker-ng doesn't know are ignored and listed in a "Config warnings" popup at startup.

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
