package ui

import (
	"fmt"
	"strings"
)

func (m *Model) viewSetup() string {
	lines := []string{"", m.sectionHead("AUDIO INPUT", plural(len(m.devices), "input")), ""}

	for index, device := range m.devices {
		mark := "   "
		name := styleInk.Render(device.Name)
		if index == m.device {
			mark = styleAccent.Render(" ▎ ")
			name = styleHeading.Render(device.Name)
		}

		note := styleFaint.Render(fmt.Sprintf("%s · %d ch · %d Hz", device.Host, device.Channels, device.Rate))
		if device.Index == m.cfg.Device {
			note = styleOk.Render("in use") + styleFaint.Render("   "+note)
		}

		lines = append(lines, pad(mark+name, note, m.width))
	}

	lines = append(lines,
		"",
		"  "+styleFaint.Render("Songs are read from "+m.cfg.Library),
		"  "+styleFaint.Render("A line in beats a microphone: it hears the guitar and not the room."),
	)

	return strings.Join(lines, "\n") + blank(m.space()-len(lines))
}
