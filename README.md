# qualisys-go

Go SDK for streaming motion capture data from Qualisys Track Manager (QTM).

Zero external dependencies — only the Go standard library.

Every example below also exists as a testable example in `example_test.go`, so
`go test` compiles them and they cannot drift from the API. They appear in
[godoc](https://pkg.go.dev/github.com/mlveggo/qualisys-go) too.

## Protocol version support

The SDK speaks RT protocol **1.28** and negotiates downwards to **1.22** when
connecting to an older QTM, mirroring the negotiation in the official
[C++ SDK](https://github.com/qualisys/qualisys_cpp_sdk). `Connect` tries the
newest version first and walks down the ladder; if QTM accepts none of them it
fails with `ErrVersionNotSupported` and closes the socket.

```go
rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
if err := rt.Connect(); err != nil {
    log.Fatal(err)
}
major, minor := rt.Version() // the version actually agreed on
```

Pin a version, or disable the fallback, with options:

```go
rt := qualisys.NewProtocol(addr, port,
    qualisys.WithVersion(1, 25),
    qualisys.WithoutVersionNegotiation(),
)
```

Because the settings XML root element is named after the negotiated version,
never hard-code it. Use `ParametersElementName` or `StripParametersElement`:

```go
xml, _ := rt.GetParameters(qualisys.ParameterTypeImage)
inner := rt.StripParametersElement(xml) // drops <QTM_Parameters_Ver_X.Y>
```

## Discovery

```go
discovery := discover.NewDiscovery(4545, 1*time.Second)
responses, err := discovery.Discover()
if err != nil {
    log.Fatal(err)
}
for _, response := range responses {
    log.Println(response)
}
```

## Streaming

```go
rt := qualisys.NewProtocol(ip, qualisys.DefaultBasePort)
if err := rt.Connect(); err != nil {
    log.Fatal(err)
}
defer rt.Disconnect()

if err := rt.StreamFramesAll(qualisys.ComponentType3D, qualisys.ComponentType6D); err != nil {
    log.Println(err) // not log.Fatal: it would skip the deferred Disconnect
    return
}

for {
    p, err := rt.Receive()
    if err != nil {
        log.Println(err)
        return
    }
    if p.EndOfData() {
        continue // nothing arrived within the read timeout
    }
    if markers := p.Data.Markers3D(); markers != nil {
        log.Printf("frame %d: %d markers", p.Data.Frame, len(markers.Markers))
    }
}
```

`DataPacket` has a typed accessor for every component: `Markers3D`,
`Markers3DResidual`, `Markers3DNoLabels`, `Markers3DNoLabelsResidual`,
`Bodies6D`, `Bodies6DResidual`, `Bodies6DEuler`, `Bodies6DEulerResidual`,
`Markers2D`, `Markers2DLinearized`, `Analog`, `AnalogSingle`, `Force`,
`ForceSingle`, `Images`, `GazeVectors`, `EyeTrackers`, `Timecodes` and
`Skeletons`. Each returns nil when the frame does not carry that component.

### Component options

Analog channel selection and global skeleton coordinates are supported:

```go
opts := qualisys.ComponentOptions{
    AnalogChannels: "1,3,5-8", // sends "Analog:1,3,5-8"
    SkeletonGlobal: true,      // sends "Skeleton:global"
}
rt.StreamFramesWithOptions(qualisys.StreamRateTypeFrequency, 100, opts,
    qualisys.ComponentTypeAnalog, qualisys.ComponentTypeSkeleton)
```

### UDP streaming

Keep commands on TCP and move the data stream to UDP:

```go
udpPort, err := rt.EnableUDPStream(0) // 0 lets the OS pick
if err != nil {
    log.Fatal(err)
}
rt.StreamFramesUDP(qualisys.StreamRateTypeAllFrames, 0, udpPort, "",
    qualisys.ComponentOptions{}, qualisys.ComponentType3D)

for {
    p, err := rt.ReceiveUDP()
    // ...
}
```

## Timeouts

Defaults are configurable per connection:

```go
rt := qualisys.NewProtocol(ip, port,
    qualisys.WithReadTimeout(2*time.Second),
    qualisys.WithConnectTimeout(10*time.Second),
    qualisys.WithMaxPacketSize(64<<20),
)
```

`Calibrate` takes its own timeout because a calibration takes minutes:

```go
resultXML, err := rt.Calibrate(false, 5*time.Minute)
```

## Error handling

Errors are wrapped and matchable with `errors.Is`:

| Sentinel                 | Meaning                                                      |
| ------------------------ | ------------------------------------------------------------ |
| `ErrNotConnected`        | Operation needs a live connection                             |
| `ErrTimeout`             | No response within the timeout                                |
| `ErrTruncated`           | Packet body never fully arrived; the stream is desynchronised |
| `ErrVersionNotSupported` | QTM accepted no version this SDK speaks                       |
| `packets.ErrShortPacket` | A component payload ended early                               |

A `Receive` timeout is *not* an error — it returns a `PacketTypeNoMoreData`
packet with a nil error. `ErrTruncated` is different and means the connection
must be re-established. `IsTimeout` covers both `ErrTimeout` and an underlying
socket deadline.

## Forward compatibility

A frame containing a component type this SDK does not recognise is still
decoded, and the undecodable component keeps its raw bytes as an
`UnknownComponent` rather than failing the whole frame. This lets a client keep
working against a newer QTM, and lets a caller who does know the format decode
it themselves.

```go
for _, unknown := range p.Data.UnknownComponentTypes() {
    // UnknownComponentData hands over the bytes; Component returns nil for
    // these, since it only surfaces components the SDK decoded itself.
    raw := p.Data.UnknownComponentData(unknown)
    log.Printf("component type %d could not be decoded, %d raw bytes kept", int(unknown), len(raw))
}
```

## Examples

```
go run ./cmd/discover
go run ./cmd/streaming -addr 192.168.0.10
go run ./cmd/streaming -udp -analog-channels 1,3,5-8
go run ./cmd/settings -addr 192.168.0.10
```

## Testing

```
go test ./...
go test -race ./...
```

Tests run entirely against an in-process fake QTM server; no hardware or
network access is required.

## Relationship to qualisys-rs

[qualisys-rs](https://github.com/mlveggo/qualisys-rs) is the sibling Rust
client. The two are kept feature-equivalent: same protocol version ladder, same
component coverage, same component and parameter options, same UDP support, and
the same treatment of undecodable components. Where the languages differ the
APIs follow local idiom — Rust models packets as an enum and returns a
connected client from `connect`, Go uses a struct with a type tag and separate
`NewProtocol`/`Connect` — but the wire behaviour is identical.

One deliberate asymmetry: this SDK ships two small XML helpers in
`pkg/settings` for pulling 3D label names and 6D body names out of a settings
response. The Rust crate exposes raw XML only, so that it keeps its single
dependency rather than pulling in an XML parser.

## Known gaps versus the C++ SDK

- Settings XML is exposed as raw strings plus a few helpers in `pkg/settings`.
  The C++ SDK has full typed (de)serialisation for General, Calibration, 3D, 6D,
  Analog, Force, Image, GazeVector, EyeTracker and Skeleton settings, including
  the typed `Set*Settings` writers.
- Protocol versions below 1.22 (including the big-endian-only 1.0 mode on the
  base port) are not implemented.
