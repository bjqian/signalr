# signalr
## Implement a golang version signalr server
It will only focus on part of its functionality. Implementing it is mainly for practicing the golang programing skills rather than the SignalR itself.
It will only support websocket transport and json protocol. No streaming and ackable message.
The key point is implementing the protocol and RPC framework.

## Refer
1. Transport protocol:https://github.com/dotnet/aspnetcore/blob/main/src/SignalR/docs/specs/TransportProtocols.md
2. Hub protocol: https://github.com/dotnet/aspnetcore/blob/main/src/SignalR/docs/specs/HubProtocol.md

WIP:
1. Add log support