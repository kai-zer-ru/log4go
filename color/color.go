package color

import (
	"strings"
)

var Bold string
var Normal string
var Faint string
var Red string
var Fail string
var Green string
var Yellow string
var Blue string
var Purple string
var RedBg string

func init() {
	_esc := func(codes ...string) string {
		return strings.Join([]string{
			"\x1b",
			"[",
			strings.Join(codes, ";"),
			"m",
		}, "")
	}
	Bold = _esc("1")
	Normal = _esc("0")
	Faint = _esc("38", "5", "240")
	Red = _esc("31", "1")
	Fail = _esc("41", "37", "1")
	Green = _esc("38", "5", "66")
	Yellow = _esc("38", "5", "220")
	Blue = _esc("38", "5", "24")
	Purple = _esc("38", "5", "96")
	RedBg = _esc("41", "1")
}
