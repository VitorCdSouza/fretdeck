package ui

// The shapes the python side sends. They are declared once here so no screen
// has to know the json of an event it only displays.

type deviceInfo struct {
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

type trackInfo struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Strings  int    `json:"strings"`
	Playable bool   `json:"playable"`
	Measures int    `json:"measures"`
}

type tracksPayload struct {
	Title  string      `json:"title"`
	Tracks []trackInfo `json:"tracks"`
}

type importedPayload struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	Notes    int    `json:"notes"`
	Measures int    `json:"measures"`
}

type progressPayload struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

type reportSummary struct {
	Notes    int     `json:"notes"`
	Hits     int     `json:"hits"`
	Accuracy float64 `json:"accuracy"`
	Extras   int     `json:"extras"`
	Missed   int     `json:"missed"`
	Duration float64 `json:"duration"`
	Tempo    float64 `json:"tempo"`
}

type reportMeasure struct {
	Index int `json:"index"`
	Notes int `json:"notes"`
	Hits  int `json:"hits"`
}

type reportNote struct {
	Kind     string   `json:"kind"`
	Measure  int      `json:"measure"`
	Time     float64  `json:"time"`
	At       float64  `json:"at"`
	Frets    [][2]int `json:"frets"`
	Expected []string `json:"expected"`
	Played   string   `json:"played"`
}

type reportPayload struct {
	Song     string          `json:"song"`
	Summary  reportSummary   `json:"summary"`
	Measures []reportMeasure `json:"measures"`
	Notes    []reportNote    `json:"notes"`
}
