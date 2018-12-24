# Foosbot

A minimal Slack bot to manage foosball games using slash commands.

The following commands are available:

| Command  | Description |
|----------|-------------|
| */new*     | _Starts a new game_ |
| */reset*   | _Reset the current game_ |
| */current* | _Who is in?_ |
| */play*    | _Join game_ |
| */giveup*  | _Leave game_ |
| */explain* | _Shows help_ |
| */ping*    | _Bot healthcheck_ |

# Installation

You need to install from a released tarball or build from the source
code.

## Using a release tarball

Download the appropriate tarball from the
[Release](https://github.com/umatrangolo/foosbot/releases) page and
install it in a folder that suits you. We encourage to create a system
only user with no login and sh that is used only to run the bot.

Once the tarball has been decompressed you should see this in the
current folder:

```shell
/home/foosbot/
├── foosbot
├── foosbot_0.1.1_Linux_x86_64.tar.gz
├── LICENSE
└── README.md
```

## Building from source

Building from source code is:

```shell
git clone git@github.com:umatrangolo/foosbot.git
cd foosbot
go install github.com/umatrangolo/foosbot
```

This will install the executable in your $GOPATH.

# Running

Foosbot is just a single executable that needs only the Slack secret
from the workspace is installed into (more later).

An ideal way to run it is to create a `foosbot` user with no password
and no sh, install the tarball into its home dir and the run in a tmux
screen:

```shell
  sudo -u foosbot SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx /home/foosbot/foosbot
```

This will start the bot listening in port 9000 for incoming slack
commands.

# Use in your ch

The app in *not distributed yet* so a bit of work is required to make
it talk with your workspace. You need to install the bot in your
ch. To accomplish this you need to go
[here](https://api.slack.com/apps) and install the bot with the
`Create New App` button. Follow the workflow and Slack will add the
bot in your workspace. After this you need to click on the app and
then on `Add features and functionality` > `Slack Commands` tabs. Here
you must add all the Slack commands the app will answer to (see
above). Use the URI of the host where your Foosbot is running and make
sure port 9000 is open there.

E.g. to setup the '/new' command fill the form with:

1. Command:           `/new`
2. Request URL:       $url-of-your-host:9000
3. Short Description: $your-description

Repeat for all the commands.

Once you done keep note of the `Signing Secret` under the `App
Credentials` section: this is the one you need to provide to the bot
as its secret.

Dublin, December 2018
