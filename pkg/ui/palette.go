package ui

// Sepia colours share one paper background, so text and accents can be checked
// against the surface on which the terminal actually displays them.
const (
	sepiaPaper          = "#E9DFCB"
	sepiaPaperDeep      = "#DDD1BA"
	sepiaSelection      = "#D8C9A8"
	sepiaBoardSelection = "#D2D5CC"
	sepiaInk            = "#3A2F22"
	sepiaInkStrong      = "#2E2418"
	sepiaInkSoft        = "#66573F"
	sepiaClosed         = "#685C47"
	sepiaNavy           = "#2F4F6F"
	sepiaInkBlue        = "#2F5878"
	sepiaMoss           = "#4A6A2A"
	sepiaOxblood        = "#8E2F2F"
	sepiaOchre          = "#7D500C"
	sepiaPlum           = "#5A3F6E"
	sepiaTeal           = "#3F6660"
	sepiaUmber          = "#6B4A2F"
	sepiaOlive          = "#5F5A14"
	sepiaMossTint       = "#D0CCB1"
	sepiaBlueTint       = "#CBC9BE"
	sepiaRedTint        = "#DAC3B2"
	sepiaOchreTint      = "#D8C8AC"
	sepiaPlumTint       = "#D2C5BC"
	sepiaTealTint       = "#CECCBA"
	sepiaClosedTint     = "#D4CAB6"
)

var sepiaAccents = [...]string{
	sepiaNavy, sepiaPlum, sepiaMoss, sepiaOchre,
	sepiaInkBlue, sepiaOxblood, sepiaTeal, sepiaUmber,
}

// PaperSequences changes the terminal's default background only for a light
// theme. OSC 111 restores the user's default when b9s exits normally.
func PaperSequences(dark bool) (set, reset string) {
	if dark {
		return "", ""
	}
	return "\x1b]11;" + sepiaPaper + "\x07", "\x1b]111\x07"
}
