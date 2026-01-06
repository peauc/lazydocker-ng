A simple terminal UI for both docker and docker-compose, written in Go with the [gocui](https://github.com/jroimartin/gocui "gocui") library.

Continuing the great work of Jesse Duffield and other contributors of [the original project](https://github.com/jesseduffield/lazydocker).

[![Go Report Card](https://goreportcard.com/badge/github.com/jesseduffield/lazydocker)](https://goreportcard.com/report/github.com/jesseduffield/lazydocker)
![GitHub repo size](https://img.shields.io/github/repo-size/peauc/lazydocker-ng)
[![GitHub Releases](https://img.shields.io/github/downloads/peauc/lazydocker-ng/total)](https://github.com/peauc/lazydocker-ng/releases)
[![GitHub tag](https://img.shields.io/github/tag/peauc/lazydocker-ng.svg)](https://github.com/peauc/lazydocker-ng/releases/latest)

![Gif](/docs/resources/demo_lazydocker_ng.gif)

[Demo](https://youtu.be/NICqQPxwJWw)

## Elevator Pitch

Minor rant incoming: Something's not working? Maybe a service is down. `docker-compose ps`. Yep, it's that microservice that's still buggy. No issue, I'll just restart it: `docker-compose restart`. Okay now let's try again. Oh wait the issue is still there. Hmm. `docker-compose ps`. Right so the service must have just stopped immediately after starting. I probably would have known that if I was reading the log stream, but there is a lot of clutter in there from other services. I could get the logs for just that one service with `docker compose logs --follow myservice` but that dies everytime the service dies so I'd need to run that command every time I restart the service. I could alternatively run `docker-compose up myservice` and in that terminal window if the service is down I could just `up` it again, but now I've got one service hogging a terminal window even after I no longer care about its logs. I guess when I want to reclaim the terminal realestate I can do `ctrl+P,Q`, but... wait, that's not working for some reason. Should I use ctrl+C instead? I can't remember if that closes the foreground process or kills the actual service.

What a headache!

Memorising docker commands is hard. Memorising aliases is slightly less hard. Keeping track of your containers across multiple terminal windows is near impossible. What if you had all the information you needed in one terminal window with every common command living one keypress away (and the ability to add custom commands as well). Lazydocker's goal is to make that dream a reality.

- [Requirements](https://github.com/peauc/lazydocker-ng#requirements)
- [Installation](https://github.com/peauc/lazydocker-ng#installation)
- [Usage](https://github.com/peauc/lazydocker-ng#usage)
- [Keybindings](/docs/keybindings)
- [Cool Features](https://github.com/peauc/lazydocker-ng#cool-features)
- [Contributing](https://github.com/peauc/lazydocker-ng#contributing)
- [Video Tutorial](https://youtu.be/NICqQPxwJWw)
- [Config Docs](/docs/Config.md)
- [FAQ](https://github.com/peauc/lazydocker-ng#faq)

## Requirements

- Docker >= **29.1.0** (API >= **1.44**)

## Installation

### Homebrew

We only offer a tap. The core package is currently pointing to the original lazydocker.

**Tap**:

```sh
brew tap peauc/lazydocker-ng
brew install peauc/lazydocker-ng/lazydocker-ng
```

### Binary Release (Linux/OSX/Windows)

You can manually download a binary release from [the release page](https://github.com/peauc/lazydocker-ng/releases).

Automated install/update, don't forget to always verify what you're piping into bash:

```sh
curl https://raw.githubusercontent.com/peauc/lazydocker-ng/master/scripts/install_update_linux.sh | bash
```

The script installs downloaded binary to `$HOME/.local/bin` directory by default, but it can be changed by setting `DIR` environment variable.

### Go

Required Go Version >= **1.19**

```sh
go install github.com/peauc/lazydocker-ng@latest
```

Required Go version >= **1.8**, <= **1.17**

```sh
go get github.com/peauc/lazydocker-ng
```

### Arch Linux AUR

You can install lazydocker using the [AUR](https://aur.archlinux.org/packages/lazydocker-ng-bin) by running:

```sh
paru -S lazydocker-ng
```

### Docker

1. <details><summary>Click if you have an ARM device</summary><p>
   - If you have a ARM 32 bit v6 architecture

   ```sh
   docker build -t lazydocker \
   --build-arg BASE_IMAGE_BUILDER=arm32v6/golang \
   --build-arg GOARCH=arm \
   --build-arg GOARM=6 \
   https://github.com/peauc/lazydocker-ng.git
   ```

   - If you have a ARM 32 bit v7 architecture

     ```sh
     docker build -t lazydocker \
     --build-arg BASE_IMAGE_BUILDER=arm32v7/golang \
     --build-arg GOARCH=arm \
     --build-arg GOARM=7 \
     https://github.com/peauc/lazydocker-ng.git
     ```

   - If you have a ARM 64 bit v8 architecture

     ```sh
     docker build -t lazydocker \
     --build-arg BASE_IMAGE_BUILDER=arm64v8/golang \
     --build-arg GOARCH=arm64 \
     https://github.com/peauc/lazydocker-ng.git
     ```

   </p></details>

1. Run the container

   ```sh
   docker run --rm -it -v \
   /var/run/docker.sock:/var/run/docker.sock \
   -v /yourpath:/.config/peauc/lazydocker-ng \
   lazydocker
   ```

   - Don't forget to change `/yourpath` to an actual path you created to store lazydocker's config
   - You can also use this [docker-compose.yml](https://github.com/peauc/lazydocker-ng/blob/master/docker-compose.yml)
   - You might want to create an alias, for example:

     ```sh
     echo "alias lzd='docker run --rm -it -v /var/run/docker.sock:/var/run/docker.sock -v /yourpath/config:/.config/peauc/lazydocker-ng lazydocker-ng'" >> ~/.zshrc
     ```

For development, you can build the image using:

```sh
git clone https://github.com/peauc/lazydocker-ng.git
cd lazydocker
docker build -t lazydocker-ng \
    --build-arg BUILD_DATE=`date -u +"%Y-%m-%dT%H:%M:%SZ"` \
    --build-arg VCS_REF=`git rev-parse --short HEAD` \
    --build-arg VERSION=`git describe --abbrev=0 --tag` \
    .
```

If you encounter a compatibility issue with Docker bundled binary, try rebuilding
the image with the build argument `--build-arg DOCKER_VERSION="v$(docker -v | cut -d" " -f3 | rev | cut -c 2- | rev)"`
so that the bundled docker binary matches your host docker binary version.

### Manual

You'll need to [install Go](https://golang.org/doc/install)

```
git clone https://github.com/peauc/lazydocker-ng.git
cd lazydocker-ng
go install
```

You can also use `go run main.go` to compile and run in one go (pun definitely intended)

## Usage

Call `lazydocker-ng` in your terminal. I personally use this a lot so I've made an alias for it like so:

```
echo "alias lzd='lazydocker-ng'" >> ~/.zshrc
```

For easy migration from the other release

```
echo "alias lazydocker='lazydocker-ng'" >> ~/.zshrc

```

(you can substitute .zshrc for whatever rc file you're using)

- Basic video tutorial [here](https://youtu.be/NICqQPxwJWw).
- List of keybindings
  [here](/docs/keybindings).

## Cool features

everything is one keypress away (or one click away! Mouse support FTW):

- viewing the state of your docker or docker-compose container environment at a glance
- viewing logs for a container/service
- viewing ascii graphs of your containers' metrics so that you can not only feel but also look like a developer
- customising those graphs to measure nearly any metric you want
- attaching to a container/service
- restarting/removing/rebuilding containers/services
- viewing the ancestor layers of a given image
- pruning containers, images, or volumes that are hogging up disk space

## Contributing

There is still a lot of work to go! Please check out the [contributing guide](CONTRIBUTING.md).
All discussions should happen in the public Github discussion tab.

## FAQ

### How do I edit my config?

By opening lazydocker, clicking on the 'project' panel in the top left, and pressing 'o' (or 'e' if your editor is vim). See [Config Docs](/docs/Config.md)

### How do I get text to wrap in my main panel?

In the future I want to make this the default, but for now there are some CPU issues that arise with wrapping. If you want to enable wrapping, use `gui.wrapMainPanel: true`

### How do you select text?

Because we support mouse events, you will need to hold option while dragging the mouse to indicate you're trying to select text rather than click on something. Alternatively you can disable mouse events via the `gui.ignoreMouseEvents` config value.

Mac Users: See [Issue #190](https://github.com/jesseduffield/lazydocker/issues/190) for other options.

### Why can't I see my container's logs?

By default we only show logs from the last hour, so that we're not putting too much strain on the machine. This may be why you can't see logs when you first start lazydocker. This can be overwritten in the config's `commandTemplates`

If you are running lazydocker in Docker container, it is a know bug, that you can't see logs or CPU usage.
