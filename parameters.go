package qualisys

import (
	"fmt"
)

//go:generate stringer -type ParameterType -trimprefix ParameterType
type ParameterType int

const (
	ParameterTypeAll ParameterType = iota
	ParameterTypeGeneral
	ParameterTypeCalibration
	ParameterType3d
	ParameterType6d
	ParameterTypeAnalog
	ParameterTypeForce
	ParameterTypeImage
	ParameterTypeGazeVector
	ParameterTypeEyeTracker
	ParameterTypeSkeleton
)

// GetParameters fetches xml settings from QTM for specified.
func (rt *RtProtocol) GetParameters(parameters ...ParameterType) (string, error) {
	parametersToString := map[ParameterType]string{
		ParameterTypeAll:         "All",
		ParameterTypeGeneral:     "General",
		ParameterTypeCalibration: "Calibration",
		ParameterType3d:          "3D",
		ParameterType6d:          "6D",
		ParameterTypeAnalog:      "Analog",
		ParameterTypeForce:       "Force",
		ParameterTypeImage:       "Image",
		ParameterTypeGazeVector:  "GazeVector",
		ParameterTypeEyeTracker:  "EyeTracker",
		ParameterTypeSkeleton:    "Skeleton",
	}
	if !rt.IsConnected() {
		return "", fmt.Errorf("getparameters: not connected")
	}
	cmd := "GetParameters"
	for _, p := range parameters {
		cmd += " " + parametersToString[p]
	}
	if err := rt.sendCommand(cmd); err != nil {
		return "", fmt.Errorf("getparameters sendcmd: %w", err)
	}
	p, err := rt.Receive()
	if err != nil {
		return "", fmt.Errorf("getparameters receive: %w", err)
	}
	return p.XMLResponse, nil
}

func (rt *RtProtocol) SetParameters(xml string) error {
	s := "<QTM_Settings>" + xml + "</QTM_Settings>"
	qtmResponses := []string{"Setting parameters succeeded"}
	if err := rt.sendAndWaitForResponse(rt.sendXML, s, qtmResponses); err != nil {
		return fmt.Errorf("setparameters sendcmd: %w", err)
	}
	return nil
}
