# goghostex

[![Workflow](https://github.com/strengthening/goghostex/workflows/build/badge.svg)](https://github.com/strengthening/goghostex)
[![License](https://img.shields.io/badge/license-BSD-blue)](https://img.shields.io/badge/license-BSD-blue)

README: [English](https://github.com/strengthening/goghostex/blob/master/README.md) | [中文](https://github.com/strengthening/goghostex/blob/master/README-zh.md)

Goghostex is a open source API of TOP crypto currency exchanges. You can use it directly for your data collector or trading program.

## Feature

The list of goghost supported API.As below:


| |SPOT|MARGIN|FUTURE|SWAP|
|:---|:---|:---|:---|:---
|OKEX|YES|YES|YES|NO|
|BINANCE|YES|NO|NO|NO


## Clone

```
git clone https://github.com/strengthening/goghostex.git
```

## Install 

```
go install
```

## Test

```
go test -v ./{package name}/... -test.run {func name}
```

## Todos

- Add `cli` features.
- Support bitmex exchange.
- Support bitstamp exchange.


## LICENSE

The project use the [New BSD License](./LICENSE)

## Credits

- [gorilla/websocket](https://github.com/gorilla/websocket)
    - A WebSocket implementation for Go.
- [nntaoli-project/GoEx](https://github.com/nntaoli-project/GoEx.git)
    - A Exchange REST and WebSocket API for Golang.