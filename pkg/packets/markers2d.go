package packets

import (
	"encoding/binary"
	"fmt"
)

type Marker2D struct {
	X         uint32
	Y         uint32
	DiameterX uint16
	DiamererY uint16
}

func (m Marker2D) String() string {
	return fmt.Sprintf("[x:%v y:%v dx:%v dy:%v]", m.X, m.Y, m.DiameterX, m.DiamererY)
}

type Camera struct {
	Markers []Marker2D
	Status  uint8
}

func (m Camera) String() string {
	return fmt.Sprintf("Status: %v Markers2D: %v", m.Markers, m.Status)
}

type Component2D struct {
	Droprate      uint16
	OutOfSyncRate uint16
	Cameras       []Camera
}

func (c Component2D) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Cameras: %v", c.Droprate, c.OutOfSyncRate, c.Cameras)
}

func (c *Component2D) UnmarshalBinary(data []byte) error {
	numberOfCameras := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Cameras = make([]Camera, 0, numberOfCameras)
	for m := uint32(0); m < numberOfCameras; m++ {
		numberOfMarkers := binary.LittleEndian.Uint32(data[pos : pos+4])
		camera := Camera{}
		camera.Status = data[pos+4]
		camera.Markers = make([]Marker2D, 0, numberOfMarkers)
		for m := uint32(0); m < numberOfMarkers; m++ {
			camera.Markers = append(camera.Markers, Marker2D{
				X:         binary.LittleEndian.Uint32(data[pos+5 : pos+9]),
				Y:         binary.LittleEndian.Uint32(data[pos+9 : pos+13]),
				DiameterX: binary.LittleEndian.Uint16(data[pos+13 : pos+15]),
				DiamererY: binary.LittleEndian.Uint16(data[pos+15 : pos+17]),
			})
			pos += 12
		}
		pos += 5
		c.Cameras = append(c.Cameras, camera)
	}
	return nil
}

type Component2DLinearized Component2D

func (c Component2DLinearized) String() string {
	return fmt.Sprintf("Droprate: %v OutOfSyncRate: %v Cameras: %v", c.Droprate, c.OutOfSyncRate, c.Cameras)
}

func (c *Component2DLinearized) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	numberOfCameras := binary.LittleEndian.Uint32(data[0:4])
	c.Droprate = binary.LittleEndian.Uint16(data[4:6])
	c.OutOfSyncRate = binary.LittleEndian.Uint16(data[6:8])
	pos := 8
	c.Cameras = make([]Camera, 0, numberOfCameras)
	for m := uint32(0); m < numberOfCameras; m++ {
		numberOfMarkers := binary.LittleEndian.Uint32(data[pos : pos+4])
		camera := Camera{}
		camera.Status = data[pos+4]
		camera.Markers = make([]Marker2D, 0, numberOfMarkers)
		for m := uint32(0); m < numberOfMarkers; m++ {
			camera.Markers = append(camera.Markers, Marker2D{
				X:         binary.LittleEndian.Uint32(data[pos+5 : pos+9]),
				Y:         binary.LittleEndian.Uint32(data[pos+9 : pos+13]),
				DiameterX: binary.LittleEndian.Uint16(data[pos+13 : pos+15]),
				DiamererY: binary.LittleEndian.Uint16(data[pos+15 : pos+17]),
			})
			pos += 12
		}
		pos += 5
		c.Cameras = append(c.Cameras, camera)
	}
	return nil
}
