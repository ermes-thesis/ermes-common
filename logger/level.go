package logger

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"os"
	"strings"
)

type Level int8

const (
	DebugLevel Level = iota
	InfoLevel
	EvalLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
	NoLevel
	Disabled

	TraceLevel Level = -1
)

func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case EvalLevel:
		return "eval"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case PanicLevel:
		return "panic"
	case Disabled:
		return "disabled"
	case NoLevel:
		return ""
	}
	return ""
}

func parseLevel(levelStr string) Level {
	switch {
	case strings.EqualFold(levelStr, TraceLevel.String()):
		return TraceLevel
	case strings.EqualFold(levelStr, DebugLevel.String()):
		return DebugLevel
	case strings.EqualFold(levelStr, InfoLevel.String()):
		return InfoLevel
	case strings.EqualFold(levelStr, EvalLevel.String()):
		return EvalLevel
	case strings.EqualFold(levelStr, WarnLevel.String()):
		return WarnLevel
	case strings.EqualFold(levelStr, ErrorLevel.String()):
		return ErrorLevel
	case strings.EqualFold(levelStr, FatalLevel.String()):
		return FatalLevel
	case strings.EqualFold(levelStr, PanicLevel.String()):
		return PanicLevel
	case strings.EqualFold(levelStr, Disabled.String()):
		return Disabled
	case strings.EqualFold(levelStr, NoLevel.String()):
		return NoLevel
	}
	return NoLevel
}

var levelColors = map[Level]int{
	TraceLevel: colorBlue,
	DebugLevel: 0,
	InfoLevel:  colorGreen,
	EvalLevel:  colorCyan,
	WarnLevel:  colorYellow,
	ErrorLevel: colorRed,
	FatalLevel: colorRed,
	PanicLevel: colorRed,
}

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorCyan = 36
)

func parseToZerologLvl(lvl Level) (zerolog.Level, error) {
	switch lvl {
	case DebugLevel:
		return zerolog.DebugLevel, nil
	case InfoLevel:
		return zerolog.InfoLevel, nil
	case EvalLevel:
		return zerolog.WarnLevel, nil
	case WarnLevel:
		return zerolog.WarnLevel, nil
	case ErrorLevel:
		return zerolog.ErrorLevel, nil
	case FatalLevel:
		return zerolog.FatalLevel, nil
	case PanicLevel:
		return zerolog.PanicLevel, nil
	case NoLevel:
		return zerolog.NoLevel, nil
	case Disabled:
		return zerolog.Disabled, nil
	case TraceLevel:
		return zerolog.TraceLevel, nil
	default:
		return 0, errors.New("unknown zerolog level")
	}
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true or c is 0.
func colorize(s string, c int, disabled bool) string {
	e := os.Getenv("NO_COLOR")
	if disabled || e != "" || c == 0 {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}
