package qualisys

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

type senderType func(string) error

func (rt *Protocol) sendCommand(cmd string) error {
	if !rt.IsConnected() {
		return fmt.Errorf("sendcommand: not connected")
	}
	const packetHeaderSize = 8
	dataSize := len(cmd) + packetHeaderSize + 1
	data := make([]byte, dataSize)
	binary.LittleEndian.PutUint32(data, uint32(dataSize))
	binary.LittleEndian.PutUint32(data[4:8], uint32(PacketTypeCommand))
	copy(data[8:dataSize], cmd+"\x00")
	if _, err := rt.conn.Write(data); err != nil {
		return fmt.Errorf("sendcommand: write failed: %w", err)
	}
	return nil
}

func (rt *Protocol) sendXML(cmd string) error {
	if !rt.IsConnected() {
		return fmt.Errorf("sendxml: not connected")
	}
	const packetHeaderSize = 8
	dataSize := len(cmd) + packetHeaderSize + 1
	data := make([]byte, dataSize)
	binary.LittleEndian.PutUint32(data, uint32(dataSize))
	binary.LittleEndian.PutUint32(data[4:8], uint32(PacketTypeXML))
	copy(data[8:dataSize], cmd+"\x00")
	if _, err := rt.conn.Write(data); err != nil {
		return fmt.Errorf("sendxml: write failed: %w", err)
	}
	return nil
}

func (rt *Protocol) SetVersion(major, minor int) error {
	ver := strconv.Itoa(major) + "." + strconv.Itoa(minor)
	cmd := "Version " + ver
	qtmResponses := []string{"Version set to " + ver}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

// GetState makes QTM send the current state as an event.
func (rt *Protocol) GetState() error {
	cmd := "GetState"
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("getstate: %w", err)
	}
	return nil
}

//go:generate stringer -type StreamRateType -trimprefix StreamRateType
type StreamRateType int

const (
	StreamRateTypeAllFrames StreamRateType = iota
	StreamRateTypeFrequency
	StreamRateTypeFrequencyDivisor
)

func (rt Protocol) getComponentString(c ComponentType) string {
	componentsToString := map[ComponentType]string{
		ComponentType3D:                 "3D",
		ComponentType3DNoLabels:         "3DNoLabels",
		ComponentTypeAnalog:             "Analog",
		ComponentTypeForce:              "Force",
		ComponentType6D:                 "6D",
		ComponentType6DEuler:            "6DEuler",
		ComponentType2D:                 "2D",
		ComponentType2DLinearized:       "2DLin",
		ComponentType3DResidual:         "3DRes",
		ComponentType3DNoLabelsResidual: "3dNoLabelsResidual",
		ComponentType6DResidual:         "6DRes",
		ComponentType6DEulerResidual:    "6DEulerRes",
		ComponentTypeAnalogSingle:       "AnalogSingle",
		ComponentTypeImage:              "Image",
		ComponentTypeForceSingle:        "ForceSingle",
		ComponentTypeGazeVector:         "GazeVector",
		ComponentTypeTimecode:           "Timecode",
		ComponentTypeSkeleton:           "Skeleton",
		ComponentTypeEyeTracker:         "EyeTracker",
	}
	return componentsToString[c]
}

func (rt *Protocol) GetCurrentFrame(components ...ComponentType) error {
	cmd := "GetCurrentFrame"
	for _, c := range components {
		cmd += " " + rt.getComponentString(c)
	}
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("getcurrentframe: %w", err)
	}
	return nil
}

func (rt *Protocol) StreamFramesAll(components ...ComponentType) error {
	if err := rt.StreamFrames(StreamRateTypeAllFrames, 0, components...); err != nil {
		return fmt.Errorf("streamframesall: %w", err)
	}
	return nil
}

func (rt *Protocol) StreamFrames(rate StreamRateType, value int, components ...ComponentType) error {
	cmd := "StreamFrames"
	switch rate {
	case StreamRateTypeAllFrames:
		cmd += " allframes"
	case StreamRateTypeFrequency:
		cmd += " frequency:" + strconv.Itoa(value)
	case StreamRateTypeFrequencyDivisor:
		cmd += " frequencydivisor:" + strconv.Itoa(value)
	}
	for _, c := range components {
		cmd += " " + rt.getComponentString(c)
	}
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("streamframes: %w", err)
	}
	return nil
}

func (rt *Protocol) StreamFramesStop() error {
	cmd := "StreamFrames stop"
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("streamframesstop: %w", err)
	}
	return nil
}

func (rt *Protocol) TakeControl(password string) error {
	cmd := "TakeControl " + password
	qtmResponses := []string{"You are now master", "You are already master"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("takecontrol: %w", err)
	}
	return nil
}

func (rt *Protocol) ReleaseControl() error {
	cmd := "ReleaseControl"
	qtmResponses := []string{"You are now a regular client", "You are already a regular client"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("releasecontrol: %w", err)
	}
	return nil
}

func (rt *Protocol) New() error {
	cmd := "New"
	qtmResponses := []string{"Creating new connection"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("new: %w", err)
	}
	return nil
}

func (rt *Protocol) Close() error {
	cmd := "Close"
	qtmResponses := []string{"Closing connection", "Closing file"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func (rt *Protocol) Start(rtFromFile bool) error {
	cmd := "Start"
	if rtFromFile {
		cmd += " RTFromFile"
	}
	qtmResponses := []string{"Starting measurement", "Starting RT from file"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func (rt *Protocol) Stop() error {
	cmd := "Stop"
	qtmResponses := []string{"Stopping measurement"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

func (rt *Protocol) Load(filename string) error {
	cmd := "Load " + filename
	qtmResponses := []string{"Measurement loaded"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("load: %w", err)
	}
	return nil
}

func (rt *Protocol) Save(filename string, overwrite bool) error {
	cmd := "Save " + filename
	if overwrite {
		cmd += " Overwrite"
	}
	qtmResponses := []string{"Measurement saved", "Measurement saved as " + filename}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	return nil
}

func (rt *Protocol) LoadProject(path string) error {
	cmd := "LoadProject " + path
	qtmResponses := []string{"Project loaded"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("loadproject: %w", err)
	}
	return nil
}

func (rt *Protocol) GetCaptureC3D() error {
	cmd := "GetCaptureC3D"
	qtmResponses := []string{"Sending capture"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("getcapturec3d: %w", err)
	}
	return nil
}

func (rt *Protocol) GetCaptureQTM() error {
	cmd := "GetCaptureQTM"
	qtmResponses := []string{"Sending capture"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("getcaptureqtm: %w", err)
	}
	return nil
}

func (rt *Protocol) Trig() error {
	cmd := "Trig"
	qtmResponses := []string{"Trig ok"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("trig: %w", err)
	}
	return nil
}

func (rt *Protocol) SetQTMEvent(label string) error {
	cmd := "SetQTMEvent " + label
	qtmResponses := []string{"Event set"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("setqtmevent: %w", err)
	}
	return nil
}

func (rt *Protocol) Reprocess() error {
	cmd := "Reprocess"
	qtmResponses := []string{"Reprocessing file"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("reprocess: %w", err)
	}
	return nil
}

func (rt *Protocol) Calibrate(refine bool) error {
	cmd := "Calibrate"
	if refine {
		cmd += " Refine"
	}
	qtmResponses := []string{"Starting calibration"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("calibrate: %w", err)
	}
	return nil
}

//go:generate stringer -type LedMode -trimprefix LedMode
type LedMode uint8

const (
	LedModeOn LedMode = iota
	LedModeOff
	LedModePulsing
)

//go:generate stringer -type LedColor -trimprefix LedColor
type LedColor uint8

const (
	LedColorAmber LedColor = iota
	LedColorGreen
	LedColorAll
)

func (rt *Protocol) Led(cameraNumber int, mode LedMode, color LedColor) error {
	cmd := "Led " + strconv.Itoa(cameraNumber) + " " + mode.String() + " " + color.String()
	if err := rt.sendCommand(cmd); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

func (rt *Protocol) Quit() error {
	cmd := "Quit"
	qtmResponses := []string{"Bye bye"}
	if err := rt.sendAndWaitForResponse(rt.sendCommand, cmd, qtmResponses); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}

func (rt *Protocol) sendAndWaitForResponse(sender senderType, s string, expectedResponses []string) error {
	if err := sender(s); err != nil {
		return fmt.Errorf("sendcommandandwaitforresponse: sender: %w", err)
	}
	p, err := rt.Receive()
	if err != nil {
		return fmt.Errorf("sendcommandandwaitforresponse: receive: %w", err)
	}
	for _, r := range expectedResponses {
		if strings.EqualFold(p.CommandResponse, r) {
			return nil
		}
	}
	return fmt.Errorf("sendcommandandwaitforresponse: response (%s)", p.CommandResponse)
}
