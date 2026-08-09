package packets

import "fmt"

type AnalogSample struct {
	Value float32
}

func (as AnalogSample) String() string { return fmt.Sprintf("[%v]", as.Value) }

type AnalogChannel struct {
	Samples []AnalogSample
}

func (ac AnalogChannel) String() string { return fmt.Sprintf("[Ch: %v]", ac.Samples) }

type AnalogDevice struct {
	ID           uint32
	SampleNumber uint32
	Channels     []AnalogChannel
}

func (ad AnalogDevice) String() string {
	return fmt.Sprintf("[id: %v samplenumber: %v channels: %v]\n", ad.ID, ad.SampleNumber, ad.Channels)
}

type ComponentAnalog struct {
	AnalogDevices []AnalogDevice
}

func (c ComponentAnalog) String() string {
	var s string
	for _, ad := range c.AnalogDevices {
		s += ad.String()
	}
	return s
}

// UnmarshalBinary decodes the multi-sample analog component.
//
// Layout per device: deviceID, channel count, sample count, sample number, then
// sampleCount samples for channel 0, then sampleCount samples for channel 1,
// and so on. Note the samples are grouped by channel, not interleaved.
func (c *ComponentAnalog) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	deviceCount := cur.Uint32()
	if !cur.checkCount(deviceCount, 16, "analog device") {
		return cur.Err()
	}

	c.AnalogDevices = make([]AnalogDevice, 0, deviceCount)
	for i := uint32(0); i < deviceCount; i++ {
		dev := AnalogDevice{ID: cur.Uint32()}
		channelCount := cur.Uint32()
		sampleCount := cur.Uint32()
		dev.SampleNumber = cur.Uint32()

		if cur.err == nil && uint64(channelCount)*uint64(sampleCount)*4 > uint64(cur.Remaining()) {
			cur.fail(fmt.Errorf("%w: analog device %d claims %d channels x %d samples",
				ErrShortPacket, dev.ID, channelCount, sampleCount))
			return cur.Err()
		}

		dev.Channels = make([]AnalogChannel, channelCount)
		for ch := uint32(0); ch < channelCount; ch++ {
			if sampleCount == 0 {
				continue
			}
			dev.Channels[ch].Samples = make([]AnalogSample, sampleCount)
			for s := uint32(0); s < sampleCount; s++ {
				dev.Channels[ch].Samples[s].Value = cur.Float32()
			}
		}
		c.AnalogDevices = append(c.AnalogDevices, dev)
	}
	return cur.Err()
}

type ComponentAnalogSingle ComponentAnalog

func (c ComponentAnalogSingle) String() string { return ComponentAnalog(c).String() }

// UnmarshalBinary decodes the single-sample analog component: one value per
// channel, no sample count or sample number field.
func (c *ComponentAnalogSingle) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	cur := newCursor(data)
	deviceCount := cur.Uint32()
	if !cur.checkCount(deviceCount, 8, "analog device") {
		return cur.Err()
	}

	c.AnalogDevices = make([]AnalogDevice, 0, deviceCount)
	for i := uint32(0); i < deviceCount; i++ {
		dev := AnalogDevice{ID: cur.Uint32()}
		channelCount := cur.Uint32()
		if !cur.checkCount(channelCount, 4, "analog channel") {
			return cur.Err()
		}
		dev.Channels = make([]AnalogChannel, channelCount)
		for ch := uint32(0); ch < channelCount; ch++ {
			dev.Channels[ch].Samples = []AnalogSample{{Value: cur.Float32()}}
		}
		c.AnalogDevices = append(c.AnalogDevices, dev)
	}
	return cur.Err()
}
