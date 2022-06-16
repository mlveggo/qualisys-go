package settings

import "encoding/xml"

type QXml struct {
	// XMLName xml.Name `xml:"QTM_Parameters_Ver_1.22"`
	Q6DXml Q6DXml `xml:"The_6D"`
	Q3DXml Q3DXml `xml:"The_3D"`
}

type Q3DXml struct {
	// XMLName xml.Name `xml:"The_6D"`
	Labels []Label `xml:"Label"`
}

type Label struct {
	Name string `xml:"Name"`
}

type Q6DXml struct {
	// XMLName xml.Name `xml:"The_6D"`
	Bodies []Body `xml:"Body"`
}

type Color struct {
	R string `xml:"R,attr"`
	G string `xml:"G,attr"`
	B string `xml:"B,attr"`
}

type Body struct {
	// XMLName xml.Name `xml:"Body"`
	Name                 string `xml:"Name"`
	Color                Color  `xml:"Color"`
	Points               Points `xml:"Points"`
	MaximumResidual      string `xml:"MaximumResidual"`
	MinimumMarkersInBody string `xml:"MinimumMarkersInBody"`
	BoneLengthTolerance  string `xml:"BoneLengthTolerance"`
	Filter               string `xml:"Filter"`
	// 	Mesh Mesh `xml:"Mesh`
}

type Points struct {
	Points []Point `xml:"Point"`
}

type Point struct {
	Name string `xml:"Name,attr"`
}

// Parse3DLabelsFromXML unmarshals 3D labels from XML string.
func Parse3DLabelsFromXML(s string) ([]string, error) {
	var qxml QXml
	if err := xml.Unmarshal([]byte(s), &qxml); err != nil {
		return nil, err
	}
	st := make([]string, 0, len(qxml.Q3DXml.Labels))
	for _, b := range qxml.Q3DXml.Labels {
		st = append(st, b.Name)
	}
	return st, nil
}

// Parse6DBodyNamesFromXML unmarshals 6D body names from XML string.
func Parse6DBodyNamesFromXML(s string) ([]string, error) {
	var qxml QXml
	if err := xml.Unmarshal([]byte(s), &qxml); err != nil {
		return nil, err
	}
	st := make([]string, 0, len(qxml.Q6DXml.Bodies))
	for _, b := range qxml.Q6DXml.Bodies {
		st = append(st, b.Name)
	}
	return st, nil
}
