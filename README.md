SORACOM Krypton client (Golang version)
=======================================

Client library and CLI tool for SORACOM Krypton.


## How to use CLI

1. Install

```
go install github.com/soracom/krypton-client-go/cmd/krypton-cli
```

or download executable file from [release page](https://github.com/soracom/krypton-client-go/releases).

On Linux, `libpcsclite` should be installed before using krypton-cli.
  Ubuntu: `sudo apt install pcscd libpcsclite1 libpcsclite-dev && sudo reboot`

2. Plug a smart card reader / USB modem with SORACOM Air SIM to your computer

3. Run

```
krypton-cli -operation getSubscriberMetadata
```

You can find other operations by using `-h` option.


## How to build from source code

```
go get -u github.com/soracom/krypton-client-go/...
cd $GOPATH/src/github.com/soracom/krypton-client-go/cmd/krypton-cli
go build
```
