package qualisys

import (
	"fmt"
	"strings"
	"time"
)

//go:generate stringer -type ParameterType -trimprefix ParameterType
type ParameterType int

const (
	ParameterTypeAll ParameterType = iota
	ParameterTypeGeneral
	ParameterTypeCalibration
	ParameterType3D
	ParameterType6D
	ParameterTypeAnalog
	ParameterTypeForce
	ParameterTypeImage
	ParameterTypeGazeVector
	ParameterTypeEyeTracker
	ParameterTypeSkeleton
)

// parameterName returns the token QTM accepts for a settings section, and
// whether the section is known at all.
func parameterName(p ParameterType) (string, bool) {
	switch p {
	case ParameterTypeAll:
		return "All", true
	case ParameterTypeGeneral:
		return "General", true
	case ParameterTypeCalibration:
		return "Calibration", true
	case ParameterType3D:
		return "3D", true
	case ParameterType6D:
		return "6D", true
	case ParameterTypeAnalog:
		return "Analog", true
	case ParameterTypeForce:
		return "Force", true
	case ParameterTypeImage:
		return "Image", true
	case ParameterTypeGazeVector:
		return "GazeVector", true
	case ParameterTypeEyeTracker:
		return "EyeTracker", true
	case ParameterTypeSkeleton:
		return "Skeleton", true
	}
	return "", false
}

// ParameterOptions carries modifiers accepted after a parameter name.
type ParameterOptions struct {
	// SkeletonGlobal requests skeleton definitions with global rather than
	// parent-relative segment transforms ("Skeleton:global").
	SkeletonGlobal bool
}

// GetParameters fetches settings XML from QTM for the requested sections.
func (rt *Protocol) GetParameters(parameters ...ParameterType) (string, error) {
	return rt.GetParametersWithOptions(ParameterOptions{}, parameters...)
}

// GetParametersWithOptions fetches settings XML with parameter modifiers.
//
// Unlike the previous implementation this skips event packets while waiting.
// QTM emits events asynchronously, so an event arriving between the request and
// the reply used to make GetParameters return an empty string with no error.
func (rt *Protocol) GetParametersWithOptions(opts ParameterOptions, parameters ...ParameterType) (string, error) {
	if !rt.IsConnected() {
		return "", fmt.Errorf("getparameters: %w", ErrNotConnected)
	}
	if len(parameters) == 0 {
		parameters = []ParameterType{ParameterTypeAll}
	}

	names := make([]string, 0, len(parameters))
	for _, p := range parameters {
		name, ok := parameterName(p)
		if !ok {
			return "", fmt.Errorf("getparameters: unknown parameter type %d", int(p))
		}
		if p == ParameterTypeSkeleton && opts.SkeletonGlobal {
			name += ":global"
		}
		names = append(names, name)
	}

	cmd := "GetParameters " + strings.Join(names, " ")
	if err := rt.sendCommand(cmd); err != nil {
		return "", fmt.Errorf("getparameters: %w", err)
	}

	deadline := time.Now().Add(DefaultCommandTimeout)
	for time.Now().Before(deadline) {
		p, err := rt.ReceiveTimeout(time.Until(deadline))
		if err != nil {
			return "", fmt.Errorf("getparameters: %w", err)
		}
		switch p.Type {
		case PacketTypeXML:
			return p.XMLResponse, nil
		case PacketTypeError:
			return "", fmt.Errorf("getparameters: %s", p.ErrorResponse)
		}
	}
	return "", fmt.Errorf("getparameters: %w waiting for XML response", ErrTimeout)
}

// SetParameters sends a settings XML fragment. The fragment is wrapped in
// <QTM_Settings> for the caller.
func (rt *Protocol) SetParameters(xml string) error {
	s := "<QTM_Settings>" + xml + "</QTM_Settings>"
	qtmResponses := []string{"Setting parameters succeeded"}
	if err := rt.sendAndWaitForResponse(rt.sendXML, s, qtmResponses); err != nil {
		return fmt.Errorf("setparameters: %w", err)
	}
	return nil
}

// StripParametersElement removes the version-specific QTM_Parameters_Ver_X.Y
// wrapper from a GetParameters response, leaving the inner fragment ready to
// pass to SetParameters.
//
// Callers used to hard-code the element name, which broke the moment the
// negotiated protocol version changed.
func (rt *Protocol) StripParametersElement(xml string) string {
	name := rt.ParametersElementName()
	xml = strings.Replace(xml, "<"+name+">", "", 1)
	xml = strings.Replace(xml, "</"+name+">", "", 1)
	return strings.TrimSpace(xml)
}
