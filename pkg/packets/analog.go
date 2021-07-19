package packets

import (
	"encoding/binary"
	"fmt"
	"math"
)

func Float32frombytes(bytes []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(bytes))
}

type AnalogSample struct {
	Value float32
}

func (as AnalogSample) String() string {
	return fmt.Sprintf("[%v]", as.Value)
}

type AnalogChannel struct {
	Samples []AnalogSample
}

func (ac AnalogChannel) String() string {
	return fmt.Sprintf("[Ch: %v]", ac.Samples)
}

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
		s += fmt.Sprintf("%v", ad.String())
	}
	return s
}

func (c *ComponentAnalog) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfAnalogDevices := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.AnalogDevices = make([]AnalogDevice, 0, numberOfAnalogDevices)
	for m := uint32(0); m < numberOfAnalogDevices; m++ {
		fp := AnalogDevice{}
		fp.ID = binary.LittleEndian.Uint32(data[pos : pos+4])
		channels := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		samples := binary.LittleEndian.Uint32(data[pos+8 : pos+12])
		fp.SampleNumber = binary.LittleEndian.Uint32(data[pos+12 : pos+16])
		fp.Channels = make([]AnalogChannel, channels)
		for ch := uint32(0); ch < channels; ch++ {
			if samples > 0 {
				fp.Channels[ch].Samples = make([]AnalogSample, samples)
				for i := uint32(0); i < samples; i++ {
					fp.Channels[ch].Samples[i].Value = Float32frombytes(data[pos+16 : pos+20])
					pos += 4
				}
			}
		}
		c.AnalogDevices = append(c.AnalogDevices, fp)
		pos += 16
	}
	return nil
}

type ComponentAnalogSingle ComponentAnalog

func (c ComponentAnalogSingle) String() string {
	return ComponentAnalog(c).String()
}

func (c *ComponentAnalogSingle) UnmarshalBinary(data []byte) error {
	numberOfAnalogDevices := binary.LittleEndian.Uint32(data[0:4])
	pos := 4
	c.AnalogDevices = make([]AnalogDevice, 0, numberOfAnalogDevices)
	for m := uint32(0); m < numberOfAnalogDevices; m++ {
		fp := AnalogDevice{}
		fp.ID = binary.LittleEndian.Uint32(data[pos : pos+4])
		channels := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		fp.Channels = make([]AnalogChannel, channels)
		for ch := uint32(0); ch < channels; ch++ {
			fp.Channels[ch].Samples = make([]AnalogSample, 1)
			fp.Channels[ch].Samples[0].Value = Float32frombytes(data[pos+8 : pos+12])
			pos += 4
		}
		c.AnalogDevices = append(c.AnalogDevices, fp)
		pos += 8
	}
	return nil
}
