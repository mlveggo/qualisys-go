package qualisys_test

// Every example in the README appears here as a testable example, so `go test`
// compiles them and they cannot drift from the API. They deliberately have no
// "Output:" comment, which means the toolchain builds them but does not run
// them -- none of them would work without a QTM on the network.
//
// They also show up in godoc alongside the functions they demonstrate, so the
// documentation and the compiled-and-checked code are the same text.
//
// Note the error handling: once Disconnect is deferred the examples log and
// return rather than calling log.Fatal, which exits the process and would skip
// every deferred call.

import (
	"errors"
	"fmt"
	"log"
	"time"

	qualisys "github.com/mlveggo/qualisys-go"
	"github.com/mlveggo/qualisys-go/pkg/discover"
)

// Connect, stream every frame and print the labeled 3D markers.
func Example() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	major, minor := rt.Version()
	fmt.Printf("negotiated protocol %d.%d\n", major, minor)

	if err := rt.StreamFramesAll(qualisys.ComponentType3D); err != nil {
		log.Println(err)
		return
	}

	for {
		p, err := rt.Receive()
		if err != nil {
			log.Println(err)
			return
		}
		// An idle socket is not an error.
		if p.EndOfData() {
			continue
		}
		if markers := p.Data.Markers3D(); markers != nil {
			fmt.Printf("frame %d: %d markers\n", p.Data.Frame, len(markers.Markers))
		}
	}
}

// Find QTM instances by UDP broadcast.
func Example_discovery() {
	discovery := discover.NewDiscovery(4545, 1*time.Second)
	responses, err := discovery.Discover()
	if err != nil {
		log.Fatal(err)
	}
	for _, response := range responses {
		fmt.Println(response)
	}
}

// Request a specific protocol version and disable the fallback ladder.
func ExampleWithVersion() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort,
		qualisys.WithVersion(1, 25),
		qualisys.WithoutVersionNegotiation(),
	)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	major, minor := rt.Version()
	fmt.Printf("%d.%d\n", major, minor)
}

// The settings XML root element is named after the negotiated protocol
// version, so it must never be hard-coded.
func ExampleProtocol_StripParametersElement() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	xml, err := rt.GetParameters(qualisys.ParameterTypeImage)
	if err != nil {
		log.Println(err)
		return
	}

	// Drops <QTM_Parameters_Ver_X.Y> for whichever version was agreed on.
	fragment := rt.StripParametersElement(xml)
	if err := rt.SetParameters(fragment); err != nil {
		log.Println(err)
		return
	}
}

// Restrict analog streaming to specific channels and request skeleton segments
// in global coordinates.
func ExampleProtocol_StreamFramesWithOptions() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	opts := qualisys.ComponentOptions{
		AnalogChannels: "1,3,5-8", // sends "Analog:1,3,5-8"
		SkeletonGlobal: true,      // sends "Skeleton:global"
	}
	if err := rt.StreamFramesWithOptions(
		qualisys.StreamRateTypeFrequency, 100, opts,
		qualisys.ComponentTypeAnalog, qualisys.ComponentTypeSkeleton,
	); err != nil {
		log.Println(err)
		return
	}
}

// Keep commands on TCP while data frames arrive over UDP.
func ExampleProtocol_StreamFramesUDP() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	udpPort, err := rt.EnableUDPStream(0) // 0 lets the OS choose
	if err != nil {
		log.Println(err)
		return
	}

	// An empty address means QTM replies to the TCP connection's address.
	if err := rt.StreamFramesUDP(
		qualisys.StreamRateTypeAllFrames, 0, udpPort, "",
		qualisys.ComponentOptions{}, qualisys.ComponentType3D,
	); err != nil {
		log.Println(err)
		return
	}

	for {
		p, err := rt.ReceiveUDP()
		if err != nil {
			log.Println(err)
			return
		}
		if p.EndOfData() {
			continue
		}
		fmt.Println(p.Data.Frame)
	}
}

// Timeouts, the packet size ceiling and byte order are per-connection.
func ExampleNewProtocol_options() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort,
		qualisys.WithReadTimeout(500*time.Millisecond),
		qualisys.WithConnectTimeout(10*time.Second),
		qualisys.WithMaxPacketSize(64<<20),
	)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()
}

// Calibration takes its own timeout because it runs for minutes.
func ExampleProtocol_Calibrate() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	if err := rt.TakeControl(""); err != nil {
		log.Println(err)
		return
	}
	calibrationXML, err := rt.Calibrate(false, 5*time.Minute)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(calibrationXML)
}

// A read timeout is not an error, but a truncated packet is unrecoverable.
func ExampleProtocol_Receive_errorHandling() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	for {
		p, err := rt.Receive()
		switch {
		case errors.Is(err, qualisys.ErrTruncated):
			// Part of the packet was consumed and the rest is still queued, so
			// the next read would misinterpret it. Nothing to do but reconnect.
			log.Println("stream desynchronised, reconnecting")
			return
		case err != nil:
			log.Println(err)
			return
		case p.EndOfData():
			// Nothing arrived within the read timeout.
			continue
		}
		fmt.Println(p.Data.Frame)
	}
}

// Components this build cannot decode keep their raw bytes rather than failing
// the whole frame.
func ExampleDataPacket_UnknownComponentTypes() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	p, err := rt.Receive()
	if err != nil {
		log.Println(err)
		return
	}
	for _, unknown := range p.Data.UnknownComponentTypes() {
		// The payload is kept, so a caller who knows the newer format can
		// decode it. Component would return nil here: it only hands back
		// components this SDK decoded itself.
		raw := p.Data.UnknownComponentData(unknown)
		fmt.Printf("QTM sent component type %d, %d raw bytes kept\n", int(unknown), len(raw))
	}
}

// Download the current capture and write it to disk.
func ExampleProtocol_SaveCaptureC3D() {
	rt := qualisys.NewProtocol("192.168.0.10", qualisys.DefaultBasePort)
	if err := rt.Connect(); err != nil {
		log.Fatal(err)
	}
	defer rt.Disconnect()

	if err := rt.SaveCaptureC3D("capture.c3d"); err != nil {
		log.Println(err)
		return
	}
}
