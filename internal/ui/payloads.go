package ui

// The shapes the python side sends. They are declared once here so no screen
// has to know the json of an event it only displays.

type deviceInfo struct {
	// ID is the name the sound server gives the input, and an index is not that
	ID string `json:"id"`

	// Card is what it is plugged into, which outlives the name: a card put in
	// another profile answers under a node name it has never been saved under
	Card     string `json:"card"`
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Channels int    `json:"channels"`
	Rate     int    `json:"rate"`
	Default  bool   `json:"default"`
}

type devicesPayload struct {
	Devices []deviceInfo `json:"devices"`
}

// listeningPayload is the input that was really opened, which is not always
// the one that was asked for.
type listeningPayload struct {
	Device int    `json:"device"`
	Source string `json:"source"`
	Card   string `json:"card"`
	Rate   int    `json:"rate"`
}

type notePayload struct {
	T     float64 `json:"t"`
	Midi  int     `json:"midi"`
	Name  string  `json:"name"`
	Freq  float64 `json:"freq"`
	Cents float64 `json:"cents"`
	Conf  float64 `json:"conf"`
	Rms   float64 `json:"rms"`
}

// levelPayload arrives twenty times a second whether a note started or not,
// which is what the tuner needs and what the meter is drawn from.
type levelPayload struct {
	T     float64 `json:"t"`
	Rms   float64 `json:"rms"`
	Freq  float64 `json:"freq"`
	Midi  int     `json:"midi"`
	Name  string  `json:"name"`
	Cents float64 `json:"cents"`
	Conf  float64 `json:"conf"`
}
