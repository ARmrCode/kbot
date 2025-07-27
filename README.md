# kbot - A simple CLI tool written in Go using the Cobra library.

# Describe
`kbot` is a version-aware command-line interface that allows you to specify the application version via a variable during compilation.

DevOps application telegramm

# Installation

Clone the repository:

```bash
git clone https://github.com/ARmrCode/kbot.git
cd kbot
go build -ldflags "-X="github.com/ARmrCode/kbot/cmd.appVersion=v1.0.0
./kbot start
