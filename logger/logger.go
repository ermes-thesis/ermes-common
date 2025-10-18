package logger

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"gopkg.in/natefinch/lumberjack.v2"
)

type outputEntry struct {
	writer   diode.Writer
	isStdout bool
}

type Logger struct {
	logger       zerolog.Logger
	outputs      []outputEntry
	config       Config
	logFile      io.WriteCloser
	childLoggers []*Logger
}

type Config struct {
	Level         zerolog.Level
	Format        string // "text" or "json"
	Output        *LogFile
	StdoutEnabled bool
	AddSource     bool
	Colored       bool
	Rolling       bool
	Muted         bool
}

type LogFile struct {
	LogDir      string
	LogFileName string
}

const levelSectionWidthWithColors = 16
const levelSectionWidthWithoutColors = 7

func makeRotatingFileWriter(logPath string) io.WriteCloser {
	return &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     7,
		Compress:   true,
	}
}

func NewLogger(config Config) (*Logger, error) {
	var (
		outputs []outputEntry
		logFile io.WriteCloser
		err     error
	)

	// Checking if an output .log file has been configured
	if config.Output != nil {
		logPath := filepath.Join(config.Output.LogDir, config.Output.LogFileName)
		err = os.MkdirAll(filepath.Dir(logPath), 0755)
		if err != nil {
			return nil, err
		}

		if config.Rolling {
			// If rolling is enabled, we use lumberjack to handle log rotation
			logFile = makeRotatingFileWriter(logPath)
		} else {
			logFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				return nil, err
			}
		}

		// If everything went well, add the file to the outputs to which the logger must write
		outputs = append(outputs, outputEntry{
			writer:   buildWriter(config.Format, logFile, true),
			isStdout: false,
		})
	}

	// If an output file has not been configured, fallback to stdout
	// If an output file has been configured, add stdout only if explicitly requested
	if len(outputs) == 0 || config.StdoutEnabled {
		outputs = append(outputs, outputEntry{
			writer:   buildConsoleWriter(os.Stdout, !config.Colored),
			isStdout: true,
		})
	}

	// Notify zerolog to use multiple writers
	var outs []io.Writer
	for _, o := range outputs {
		outs = append(outs, o.writer)
	}
	writer := zerolog.MultiLevelWriter(outs...)

	// Initializing the base structure of the logger with a timestamp
	base := zerolog.New(writer).With().Timestamp()

	// If requested add the Caller file:line
	if config.AddSource {
		base = base.CallerWithSkipFrameCount(3)
	}

	// Create the logger
	logger := base.Logger().Level(config.Level)
	return &Logger{
		logger:  logger,
		outputs: outputs,
		config:  config,
		logFile: logFile,
	}, nil
}

// colorize returns the string s wrapped in ANSI code c, unless disabled is true or c is 0.
func colorize(s string, c int, disabled bool) string {
	e := os.Getenv("NO_COLOR")
	if disabled || e != "" || c == 0 {
		return fmt.Sprintf("%s", s)
	}
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

// buildConsoleWriter wraps the parameter writer into a zerolog.ConsoleWriter to
// create some pretty logging. This is the default writer that is used to write to
// os.Stdout
func buildConsoleWriter(out io.Writer, noColor bool) diode.Writer {
	return diode.NewWriter(zerolog.ConsoleWriter{
		Out:           out,
		NoColor:       noColor,
		TimeFormat:    "2006-01-02 15:04:05",
		FieldsExclude: []string{"component"},
		PartsOrder:    []string{zerolog.LevelFieldName, zerolog.TimestampFieldName, "component", zerolog.CallerFieldName, zerolog.MessageFieldName},
		FormatPartValueByName: func(i any, s string) string {
			if s == "component" {
				value, ok := i.(string)
				if !ok {
					return ""
				}
				return strings.ToUpper(fmt.Sprintf("[%s]", value))
			}
			return i.(string)
		},
		FormatFieldName: func(i any) string {
			str, ok := i.(string)
			if !ok {
				return ""
			}
			return "| " + str + "="
		},
		FormatLevel: func(levelStr any) string {
			uppercaseLevel := strings.ToUpper(levelStr.(string))
			level, _ := zerolog.ParseLevel(uppercaseLevel)
			levelSection := colorize(fmt.Sprintf("[%s]", uppercaseLevel), zerolog.LevelColors[level], noColor)
			pad := levelSectionWidthWithColors - len(levelSection)
			if noColor {
				pad = levelSectionWidthWithoutColors - len(levelSection)
			}
			levelSection = levelSection + strings.Repeat(" ", pad)
			return levelSection
		},
	}, 1000, 10*time.Millisecond, func(missed int) {
		fmt.Printf("Logger dropped %d messages", missed)
	})
}

// buildWriter is used to build a writer according to the specified format.
//
// If the "json" format is chosen, the parameter writer is returned as it is, and zerolog
// will take care of its formatting (JSON, is the default formatting for zerolog logging)
//
// If the "text" format is chosen, the parameter writer is wrapper in a zerolog.ConsoleWriter
// that will take care of formatting the logging messages in a pretty and easily understandable way
func buildWriter(format string, out io.Writer, noColor bool) diode.Writer {
	if format == "json" {
		return diode.NewWriter(out, 1000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("Logger dropped %d messages", missed)
		})
	}

	return buildConsoleWriter(out, noColor)
}

func (l *Logger) Close() error {
	for _, child := range l.childLoggers {
		err := child.Close()
		if err != nil {
			return err
		}
	}

	for _, o := range l.outputs {
		if o.isStdout {
			continue
		}
		err := o.writer.Close()
		if err != nil {
			return err
		}
	}

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

func (l *Logger) Info(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Info().Fields(attrs).Msg(msg)
	}
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Info().Msg(fmt.Sprintf(format, args...))
	}
}

func (l *Logger) InfofWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Info().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Logger) Error(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msg(msg)
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Error().Msgf(format, args...)
	}
}

func (l *Logger) ErrorfErr(format string, args ...interface{}) error {
	errStr := fmt.Sprintf(format, args...)
	if !l.config.Muted {
		l.logger.Error().Msg(errStr)
	}
	return errors.New(errStr)
}

func (l *Logger) Debug(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Debug().Fields(attrs).Msg(msg)
	}
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Debug().Msgf(format, args...)
	}
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	if !l.config.Muted {
		l.logger.Warn().Msgf(format, args...)
	}
}

func (l *Logger) DebugfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Debug().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Logger) Warn(msg string, attrs ...any) {
	if !l.config.Muted {
		l.logger.Warn().Fields(attrs).Msg(msg)
	}
}

func (l *Logger) WarnfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Warn().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Logger) ErrorfWith(format string, args []any, attrs ...any) {
	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msgf(format, args...)
	}
}

func (l *Logger) ErrorfErrWith(format string, args []any, attrs ...any) error {
	errStr := fmt.Sprintf(format, args...)

	if !l.config.Muted {
		l.logger.Error().Fields(attrs).Msg(errStr)
	}
	return errors.New(errStr)
}

func (l *Logger) Enabled(level zerolog.Level) bool {
	return level >= l.logger.GetLevel()
}

func (l *Logger) Mute() {
	l.config.Muted = true
}

func (l *Logger) Unmute() {
	l.config.Muted = false
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		logger: l.logger.With().Fields(args).Logger(),
		config: l.config,
	}
}

func (l *Logger) Component(name string) *Logger {
	return &Logger{
		logger: l.logger.With().Str("component", name).Logger(),
		config: l.config,
	}
}

func (l *Logger) ComponentWithFile(name string) (*Logger, error) {
	if l.config.Output == nil {
		return nil, fmt.Errorf("cannot create separated log file for component %s: no base log file configured", name)
	}

	newLogPath := filepath.Join(l.config.Output.LogDir, fmt.Sprintf("%s.log", name))

	var logFile io.WriteCloser
	var err error
	if l.config.Rolling {
		// If rolling is enabled, we use lumberjack to handle log rotation
		logFile = makeRotatingFileWriter(newLogPath)
	} else {
		logFile, err = os.OpenFile(newLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return nil, err
		}
	}
	newWriter := buildWriter(l.config.Format, logFile, true)

	// Notify zerolog to use multiple writers
	var outs []io.Writer
	for _, o := range l.outputs {
		outs = append(outs, o.writer)
	}
	outs = append(outs, newWriter)
	writer := zerolog.MultiLevelWriter(outs...)

	// Initializing the base structure of the logger with a timestamp
	base := zerolog.New(writer).With().Timestamp().Str("component", name)

	// If requested add the Caller file:line
	if l.config.AddSource {
		base = base.CallerWithSkipFrameCount(3)
	}

	// Create the logger
	logger := base.Logger().Level(l.config.Level)

	childLogger := &Logger{
		logger: logger,
		outputs: []outputEntry{
			{
				writer:   newWriter,
				isStdout: false,
			},
		},
		config:  l.config,
		logFile: logFile,
	}

	l.childLoggers = append(l.childLoggers, childLogger)

	return childLogger, nil
}
